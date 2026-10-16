package featureflag

import (
	"context"
	"fmt"
	"io"
	"net/http"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
)

// Client implements port.FlagClient over OpenFeature + Unleash with a
// Valkey/in-memory fallback. It is safe for concurrent use; all dependencies
// are private and set once at construction (Parameter Object pattern), so all
// methods use pointer receivers.
type Client struct {
	evaluator    Evaluator
	fallback     *Fallback
	egress       EgressWaiter
	breaker      kernelresilience.Breaker
	http         httpclient.Doer
	retryPolicy  kernelresilience.RetryPolicy
	unleashURL   string
	probeMaxBody int
	logger       log.Logger
}

// Compile-time port conformance.
var _ appport.FlagClient = (*Client)(nil)

// NewClient builds the flag adapter. Params are validated deterministically;
// a nil Evaluator selects the production Unleash evaluator wired through the
// egress-gated HTTP client. RefreshInterval must be explicit (10s per epic).
func NewClient(params Params) (*Client, error) {
	if err := params.Validate(); err != nil {
		return nil, err
	}

	fallback := params.Fallback
	if fallback == nil {
		var err error

		fallback, err = NewFallback(FallbackParams{
			Clock:           params.Clock,
			RefreshInterval: params.RefreshInterval,
			Defaults:        params.Defaults,
			Cache:           params.Cache,
			CacheTTL:        params.CacheTTL,
			Logger:          params.Logger,
		})
		if err != nil {
			return nil, err
		}
	}

	evaluator := params.Evaluator
	if evaluator == nil {
		gated, err := NewGatedTransport(params.Egress, params.Breaker, params.Transport, params.Logger)
		if err != nil {
			return nil, err
		}

		httpClient := &http.Client{Transport: gated}

		evaluator, err = NewUnleashEvaluator(UnleashEvaluatorParams{
			UnleashURL:      params.UnleashURL,
			AppName:         params.AppName,
			InstanceID:      params.InstanceID,
			Environment:     params.Environment,
			RefreshInterval: params.RefreshInterval,
			ClientToken:     params.ClientToken,
			InitTimeout:     params.InitTimeout,
			HTTPClient:      httpClient,
		})
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		evaluator:    evaluator,
		fallback:     fallback,
		egress:       params.Egress,
		breaker:      params.Breaker,
		http:         params.HTTP,
		retryPolicy:  params.RetryPolicy,
		unleashURL:   params.UnleashURL,
		probeMaxBody: params.ProbeMaxBodyBytes,
		logger:       params.Logger,
	}, nil
}

// Boolean evaluates one boolean flag, returning the configured default with a
// typed error when the provider is unreachable.
func (c *Client) Boolean(ctx context.Context, key string, def bool, evalCtx appport.FlagContext) (bool, error) {
	details, err := c.BooleanWithDetails(ctx, key, def, evalCtx)

	return details.Value, err
}

// BooleanWithDetails evaluates one boolean flag with provenance markers.
// Provider success returns fresh values and records last-known; provider or
// breaker failures serve fallback with the typed failure alongside the value.
// Limiter failures never occur here (evaluations are local); they fail closed
// in Refresh and the gated background transport without an unmetered request.
func (c *Client) BooleanWithDetails(
	ctx context.Context,
	key string,
	def bool,
	evalCtx appport.FlagContext,
) (appport.FlagDetails, error) {
	if c == nil || c.evaluator == nil || c.fallback == nil || c.breaker == nil {
		return appport.FlagDetails{Value: def, Fallback: true}, newConfigError("featureflag: client is not initialized")
	}

	if key == "" {
		return appport.FlagDetails{Value: def, Fallback: true}, entity.NewError(CodeInvalidFlagKey, "featureflag: flag key is required")
	}

	if err := ctx.Err(); err != nil {
		value, _, _ := c.fallback.ResolveWithContext(ctx, key, def)

		return appport.FlagDetails{Value: value, Fallback: true}, err
	}

	value, err := c.evaluateWithResilience(ctx, key, def, evalCtx)
	if err == nil {
		c.fallback.Observe(ctx, key, value)

		return appport.FlagDetails{Value: value}, nil
	}

	if isBreakerOpen(err) {
		value, _, stale := c.fallback.ResolveWithContext(ctx, key, def)

		return appport.FlagDetails{Value: value, Fallback: true, Stale: stale}, entity.NewError(
			CodeBreakerOpen,
			"featureflag: breaker open, serving fallback",
		)
	}

	value, _, stale := c.fallback.ResolveWithContext(ctx, key, def)

	return appport.FlagDetails{Value: value, Fallback: true, Stale: stale}, err
}

// Refresh probes Unleash connectivity through the egress quota and the
// breaker with bounded retries. Every attempt (initial + retries) acquires a
// permit before the HTTP request; limiter-store errors fail closed without an
// unmetered request and without counting a breaker failure. It is the
// explicit egress proof for jobs and tests; background toggle refreshes flow
// through the same gated transport.
func (c *Client) Refresh(ctx context.Context) error {
	if c == nil || c.egress == nil || c.breaker == nil || c.http == nil {
		return newConfigError("featureflag: client is not initialized")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := c.retryPolicy.Validate(); err != nil {
		return err
	}

	var lastErr error

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := c.egress.Wait(ctx, EgressProvider, EgressOperation); err != nil {
			// Terminal: no HTTP attempt, no breaker count, no retry.
			return err
		}

		var probeErr error

		breakerErr := c.breaker.Execute(ctx, "unleash-refresh", "", func(callCtx context.Context) error {
			probeErr = c.probeOnce(callCtx)

			return probeErr
		})
		if isBreakerOpen(breakerErr) {
			return entity.NewError(CodeBreakerOpen, "featureflag: breaker open during refresh")
		}

		// Breaker closed: breakerErr mirrors probeErr. Success returns;
		// failures retry per classification below.
		if probeErr == nil {
			return nil
		}

		lastErr = probeErr
		if breakerErr != nil {
			lastErr = breakerErr
		}

		if attempt >= c.retryPolicy.MaxAttempts {
			break
		}

		if lastErr != nil && !kernelresilience.ShouldRetry(kernelresilience.DefaultClassifier(lastErr), true, "flag-refresh") {
			break
		}

		backoff, err := c.retryPolicy.BackoffFor(attempt)
		if err != nil {
			return err
		}

		if err := infraresilience.RealSleep(ctx, backoff); err != nil {
			return err
		}
	}

	if lastErr == nil {
		return newProviderUnavailableError()
	}

	return fmt.Errorf("featureflag: refresh attempts exhausted: %w", lastErr)
}

// Health reports provider readiness without emitting credentials. A ready
// provider with fresh fallback is healthy; otherwise callers must treat flags
// as fallback hints.
func (c *Client) Health(ctx context.Context) error {
	if c == nil || c.evaluator == nil {
		return newConfigError("featureflag: client is not initialized")
	}

	return c.evaluator.Health(ctx)
}

// Close releases provider resources. It is safe to call more than once.
func (c *Client) Close(ctx context.Context) error {
	if c == nil || c.evaluator == nil {
		return nil
	}

	return c.evaluator.Close(ctx)
}

// evaluateWithResilience runs one provider evaluation inside the breaker with
// bounded classified retries. Flag reads are idempotent cacheable reads, so
// retryable and unknown failures retry; permanent failures and cancellation
// never retry. Breaker-open fast-fails without calling the provider.
func (c *Client) evaluateWithResilience(
	ctx context.Context,
	key string,
	def bool,
	evalCtx appport.FlagContext,
) (bool, error) {
	if err := c.retryPolicy.Validate(); err != nil {
		return def, err
	}

	var lastErr error

	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return def, err
		}

		var (
			value   bool
			callErr error
		)

		breakerErr := c.breaker.Execute(ctx, "flag-evaluate", "", func(callCtx context.Context) error {
			value, callErr = c.evaluator.Evaluate(callCtx, key, def, evalCtx)

			return callErr
		})
		if isBreakerOpen(breakerErr) {
			// Breaker-open fast-fails: never retry as a provider error and
			// never count limiter behavior here (no egress on this path).
			return def, breakerErr
		}

		// Breaker closed: breakerErr mirrors callErr (or is nil on success).
		// Provider failures retry per classification below.
		if callErr == nil {
			return value, nil
		}

		lastErr = callErr

		if attempt >= c.retryPolicy.MaxAttempts {
			break
		}

		if !kernelresilience.ShouldRetry(kernelresilience.DefaultClassifier(callErr), true, "flag-read") {
			break
		}

		backoff, err := c.retryPolicy.BackoffFor(attempt)
		if err != nil {
			return def, err
		}

		if err := infraresilience.RealSleep(ctx, backoff); err != nil {
			return def, err
		}
	}

	return def, fmt.Errorf("featureflag: evaluation attempts exhausted: %w", lastErr)
}

// probeOnce performs one egress-admitted HTTP probe. Non-2xx statuses map to
// classified errors (5xx/429 retryable, 401/403 permanent, others retryable as
// unknown-outcome); transport errors are retryable. Response bodies are
// drained and closed; no body, header, or credential is logged.
func (c *Client) probeOnce(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.unleashURL, nil)
	if err != nil {
		return &kernelresilience.RetryableError{Err: fmt.Errorf("%w: %w", ErrRefreshProbeFailed, err)}
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &kernelresilience.RetryableError{Err: fmt.Errorf("%w: %w", ErrRefreshProbeFailed, err)}
	}

	defer func() {
		// Drain and close failures are logged, never ignored; the probe
		// outcome itself is already decided by the status code above.
		if _, err := io.Copy(io.Discard, io.LimitReader(resp.Body, int64(c.probeMaxBody))); err != nil && c.logger != nil {
			c.logger.Warn(ctx, "featureflag: probe body drain failed", "cause", err)
		}

		if err := resp.Body.Close(); err != nil && c.logger != nil {
			c.logger.Warn(ctx, "featureflag: probe body close failed", "cause", err)
		}
	}()

	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &kernelresilience.PermanentError{
			Err: fmt.Errorf("%w: status %d", ErrRefreshProbeFailed, resp.StatusCode),
		}
	default:
		return &kernelresilience.RetryableError{
			Err: fmt.Errorf("%w: status %d", ErrRefreshProbeFailed, resp.StatusCode),
		}
	}
}
