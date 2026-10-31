package fx

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	kernelres "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// GetRate returns the freshest rate the adapter holds. A fresh cache entry is
// served without any egress or HTTP work. A miss or expired entry refreshes
// through the coalescer with one egress permit per actual HTTP attempt. When
// the refresh cannot run, a valid last-known rate is returned unchanged with
// a nil error so callers can observe IsStale; with no cached rate the failure
// is a typed error.
func (p *Provider) GetRate(
	ctx context.Context,
	base, quote valueobject.AssetCode,
	at time.Time,
) (valueobject.FxRate, error) {
	base = valueobject.AssetCode(strings.TrimSpace(string(base)))
	quote = valueobject.AssetCode(strings.TrimSpace(string(quote)))

	pair, err := valueobject.NewFxPair(base, quote)
	if err != nil {
		return valueobject.FxRate{}, entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes")
	}

	if err := ctx.Err(); err != nil {
		return valueobject.FxRate{}, err
	}

	key, err := service.BuildFXRateKey(base, quote)
	if err != nil {
		return valueobject.FxRate{}, err
	}

	now := p.clock.Now()

	effectiveAt := at
	if effectiveAt.IsZero() {
		effectiveAt = now
	}

	if effectiveAt.After(now) {
		effectiveAt = now
	}

	cached, cacheErr := p.cache.Get(ctx, key)

	cachedValid := cacheErr == nil && cached.Validate() == nil
	if cachedValid && !cached.IsStale(effectiveAt) {
		return cached, nil
	}

	payload, err := p.coalescer.Do(ctx, coalesce.Key{
		Provider:      egressProvider,
		Scope:         coalesceScope,
		RequestDigest: key,
		Kind:          coalesce.CacheableFXRate,
	}, func(sharedCtx context.Context) ([]byte, error) {
		fetched, fetchErr := p.refresh(sharedCtx, base, quote)
		if fetchErr != nil {
			return nil, fetchErr
		}

		// The complete rate is persisted with physical retention beyond its
		// logical TTL, so a last-known value survives expiry. A cache write
		// failure is best-effort: the fresh fetch is still authoritative.
		_ = p.cache.Set(sharedCtx, key, fetched, p.staleRetention)

		encoded, encodeErr := jsonparser.Marshal(fetched)
		if encodeErr != nil {
			return nil, &kernelres.PermanentError{
				Err: entity.NewError(CodeInvalidResponse, "fx: rate could not be encoded"),
			}
		}

		return encoded, nil
	})
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return valueobject.FxRate{}, ctxErr
		}

		if cachedValid {
			return cached, nil
		}

		return valueobject.FxRate{}, normalizeFetchError(err)
	}

	var rate valueobject.FxRate
	if err := jsonparser.Unmarshal(payload, &rate); err != nil {
		if cachedValid {
			return cached, nil
		}

		return valueobject.FxRate{}, entity.NewError(CodeInvalidResponse, "fx: fetched rate could not be decoded")
	}

	if err := rate.Validate(); err != nil {
		if cachedValid {
			return cached, nil
		}

		return valueobject.FxRate{}, entity.NewError(CodeRateInvalid, "fx: fetched rate is invalid")
	}

	if rate.Pair != pair {
		if cachedValid {
			return cached, nil
		}

		return valueobject.FxRate{}, entity.NewError(CodeRateInvalid, "fx: fetched rate pair does not match the request")
	}

	return rate, nil
}

// refresh runs the bounded retry loop. Each attempt checks the breaker before
// spending a permit, acquires exactly one egress permit, and then performs one
// breaker-wrapped HTTP attempt. An already-open breaker performs no egress or
// HTTP work.
func (p *Provider) refresh(
	ctx context.Context,
	base, quote valueobject.AssetCode,
) (valueobject.FxRate, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, p.fetchTimeout)
	defer cancel()

	var rate valueobject.FxRate

	attempt := func(attemptCtx context.Context) error {
		if p.breaker.State() == kernelres.StateOpen {
			return &kernelres.PermanentError{
				Err: entity.NewError(CodeProviderUnavailable, msgBreakerOpen),
			}
		}

		if err := p.egress.Wait(attemptCtx, egressProvider, egressOperation); err != nil {
			return &kernelres.PermanentError{Err: egressFailure(err)}
		}

		return p.breaker.Execute(attemptCtx, egressOperation, "", func(callCtx context.Context) error {
			fetched, err := p.fetch(callCtx, base, quote)
			if err != nil {
				return err
			}

			rate = fetched

			return nil
		})
	}

	err := infraresilience.Execute(
		fetchCtx,
		infraresilience.RealSleep,
		p.retryPolicy,
		true,
		"",
		classifyFailure,
		attempt,
	)
	if err != nil {
		return valueobject.FxRate{}, normalizeFetchError(err)
	}

	return rate, nil
}

// fetch performs one HTTP GET /live attempt. Transport, 5xx, 429, and body
// failures are retryable; 4xx and unusable payloads are permanent. No
// provider error text, URL, body, or credential is attached to the error.
func (p *Provider) fetch(
	ctx context.Context,
	base, quote valueobject.AssetCode,
) (valueobject.FxRate, error) {
	endpoint := *p.endpoint
	query := endpoint.Query()
	query.Set("access_key", p.apiKey)
	query.Set("source", string(base))
	query.Set("currencies", string(quote))
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return valueobject.FxRate{}, &kernelres.PermanentError{
			Err: entity.NewError(CodeInvalidConfig, msgRequestFailed),
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return valueobject.FxRate{}, &kernelres.RetryableError{
			Err: entity.NewError(CodeProviderUnavailable, msgRequestFailed),
		}
	}

	defer func() { _ = resp.Body.Close() }()

	maxRead := p.maxResponseBytes
	if maxRead < math.MaxInt64 {
		maxRead++
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRead))
	if err != nil {
		return valueobject.FxRate{}, &kernelres.RetryableError{
			Err: entity.NewError(CodeProviderUnavailable, msgResponseReadFailed),
		}
	}

	if int64(len(body)) > p.maxResponseBytes {
		return valueobject.FxRate{}, &kernelres.RetryableError{
			Err: entity.NewError(CodeProviderUnavailable, msgResponseReadFailed),
		}
	}

	switch {
	case resp.StatusCode == http.StatusOK:
		rate, parseErr := parseLiveRate(body, base, quote, p.source, p.clock.Now(), parsePolicy{
			TTL:           p.ttl,
			MaxQuoteAge:   p.maxQuoteAge,
			MaxFutureSkew: p.maxFutureSkew,
			MaxDecimalLen: p.maxDecimalLen,
		})
		if parseErr != nil {
			return valueobject.FxRate{}, &kernelres.PermanentError{Err: parseErr}
		}

		return rate, nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError:
		return valueobject.FxRate{}, &kernelres.RetryableError{
			Err: entity.NewError(CodeProviderUnavailable, msgProviderUnavailable),
		}
	default:
		return valueobject.FxRate{}, &kernelres.PermanentError{
			Err: entity.NewError(CodeProviderRejected, msgProviderRejected),
		}
	}
}

// classifyFailure treats an open breaker as terminal; all other classes come
// from the retry wrappers produced by fetch.
func classifyFailure(err error) kernelres.Class {
	if errors.Is(err, infraresilience.ErrBreakerOpen) {
		return kernelres.Permanent
	}

	return kernelres.DefaultClassifier(err)
}

// normalizeFetchError reduces retry/breaker wrappers to one stable typed
// failure. Caller cancellation is handled by GetRate before this point.
func normalizeFetchError(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return entity.NewError(CodeProviderUnavailable, msgFetchTimedOut)
	}

	if errors.Is(err, infraresilience.ErrBreakerOpen) {
		return entity.NewError(CodeProviderUnavailable, msgBreakerOpen)
	}

	coded, ok := errors.AsType[*entity.Error](err)
	if ok {
		return coded
	}

	return entity.NewError(CodeProviderUnavailable, msgFetchFailed)
}

// egressFailure maps an admission denial or limiter-store failure to a
// typed failure while retaining the underlying egress code for diagnosis.
func egressFailure(err error) *entity.Error {
	if coded, ok := errors.AsType[*entity.Error](err); ok {
		return entity.NewError(CodeEgressUnavailable, "fx: provider egress admission failed ("+coded.Code+")")
	}

	return entity.NewError(CodeEgressUnavailable, "fx: provider egress admission failed")
}
