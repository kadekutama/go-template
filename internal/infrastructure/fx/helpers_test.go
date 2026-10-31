package fx

import (
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	kernelres "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockegress "github.com/kadekutama/go-template/test/mock/egress"
	mockfx "github.com/kadekutama/go-template/test/mock/fx"
	mockhttpclient "github.com/kadekutama/go-template/test/mock/httpclient"
	mockresilience "github.com/kadekutama/go-template/test/mock/resilience"
)

const (
	testSource = "exchangerate.host"
	testAPIKey = "secret-key-value"
)

func testNow() time.Time {
	return time.Date(2026, 10, 28, 12, 0, 0, 0, time.UTC)
}

func testRetryPolicy() kernelres.RetryPolicy {
	return kernelres.RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     2.0,
	}
}

// testParams builds an independent happy-path fixture. Every dependency is a
// mockery-generated mock, so table cases can replace one seam without sharing
// state with sibling subtests.
func testParams() Params {
	clock := &mockapplication.MockClock{}
	clock.On("Now").Return(testNow())

	cache := &mockfx.MockRateCache{}
	cache.On("Get", mock.Anything, mock.Anything).Return(valueobject.FxRate{}, hybrid.ErrCacheMiss)
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	egress := &mockegress.MockWaiter{}
	egress.On("Wait", mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	breaker := &mockresilience.MockBreaker{}
	breaker.On("State").Return(kernelres.StateClosed).Maybe()
	breaker.On("Execute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, _ string, _ string, fn func(context.Context) error) error {
			return fn(ctx)
		}, nil).
		Maybe()

	coalescer := &mockfx.MockCoalescer{}
	coalescer.On("Do", mock.Anything, mock.Anything, mock.Anything).
		Return(func(ctx context.Context, _ coalesce.Key, fn func(context.Context) ([]byte, error)) ([]byte, error) {
			return fn(ctx)
		}, nil).
		Maybe()

	doer := &mockhttpclient.MockDoer{}
	doer.On("Do", mock.Anything).
		Return(func(req *http.Request) (*http.Response, error) {
			return testResponse(req, http.StatusOK, string(liveBody(true, "USD", "USDEUR", "0.92", testNow().Unix()))), nil
		}, nil).
		Maybe()

	return Params{
		BaseURL:          "https://fx.example.test/",
		APIKey:           testAPIKey,
		Source:           testSource,
		LivePath:         "/live",
		HTTPClient:       doer,
		Cache:            cache,
		Egress:           egress,
		Breaker:          breaker,
		Coalescer:        coalescer,
		Clock:            clock,
		RetryPolicy:      testRetryPolicy(),
		FetchTimeout:     5 * time.Second,
		MaxResponseBytes: 1 << 20,
		MaxFutureSkew:    5 * time.Minute,
		MaxDecimalLen:    40,
		TTL:              time.Hour,
		StaleRetention:   24 * time.Hour,
		MaxQuoteAge:      48 * time.Hour,
	}
}

func freshCachedRate() valueobject.FxRate {
	return valueobject.FxRate{
		ID:             "fx:USD:EUR:fresh",
		Pair:           valueobject.FxPair{Base: "USD", Quote: "EUR"},
		Numerator:      91,
		Denominator:    100,
		Source:         testSource,
		QuotedAt:       testNow().Add(-10 * time.Minute),
		TTL:            time.Hour,
		RoundingPolicy: roundingPolicy,
	}
}

func staleCachedRate() valueobject.FxRate {
	return valueobject.FxRate{
		ID:             "fx:USD:EUR:stale",
		Pair:           valueobject.FxPair{Base: "USD", Quote: "EUR"},
		Numerator:      22,
		Denominator:    25,
		Source:         testSource,
		QuotedAt:       testNow().Add(-2 * time.Hour),
		TTL:            time.Hour,
		RoundingPolicy: roundingPolicy,
	}
}

func liveBody(success bool, source, quoteKey, quoteValue string, timestamp int64) []byte {
	successValue := "false"
	if success {
		successValue = "true"
	}

	return []byte(`{"success":` + successValue + `,"source":"` + source +
		`","quotes":{"` + quoteKey + `":` + quoteValue + `},"timestamp":` +
		strconv.FormatInt(timestamp, 10) + `}`)
}

func expectedLiveRate(now time.Time, quote valueobject.AssetCode, numerator, denominator int64) valueobject.FxRate {
	return valueobject.FxRate{
		ID:             "fx:USD:" + string(quote) + ":" + strconv.FormatInt(now.UnixNano(), 10),
		Pair:           valueobject.FxPair{Base: "USD", Quote: quote},
		Numerator:      numerator,
		Denominator:    denominator,
		Source:         testSource,
		QuotedAt:       now,
		TTL:            time.Hour,
		RoundingPolicy: roundingPolicy,
	}
}

// callCount counts recorded invocations of one method on a generated mock.
// Method-specific key/TTL inspectors stay separate.
func callCount(m *mock.Mock, method string) int {
	count := 0

	for _, call := range m.Calls {
		if call.Method == method {
			count++
		}
	}

	return count
}

// testResponse builds a fresh response for one Do call; retries must not share
// a body reader.
func testResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}

// doerWithResponse scripts every Do call with the same status and body.
func doerWithResponse(status int, body string) *mockhttpclient.MockDoer {
	return doerPerRequest(func(req *http.Request) (*http.Response, error) {
		return testResponse(req, status, body), nil
	})
}

// doerWithError scripts every Do call with the same transport failure.
func doerWithError(err error) *mockhttpclient.MockDoer {
	return doerPerRequest(func(*http.Request) (*http.Response, error) {
		return nil, err
	})
}

// doerBlockedUntilContext scripts every Do call to wait for cancellation and
// then report the request-context failure.
func doerBlockedUntilContext() *mockhttpclient.MockDoer {
	return doerPerRequest(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()

		return nil, req.Context().Err()
	})
}

// doerPerRequest scripts Do from the request itself (used by the coalescing
// proof, where each quote needs its own body).
func doerPerRequest(respond func(*http.Request) (*http.Response, error)) *mockhttpclient.MockDoer {
	doer := &mockhttpclient.MockDoer{}
	doer.On("Do", mock.Anything).Return(respond, nil)

	return doer
}

// cacheWith scripts a cache Get result. Set is allowed but optional so the
// same helper serves cache-hit cases.
func cacheWith(rate valueobject.FxRate, err error) *mockfx.MockRateCache {
	cache := &mockfx.MockRateCache{}
	cache.On("Get", mock.Anything, mock.Anything).Return(rate, err)
	cache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	return cache
}

// breakerOpen scripts the breaker pre-check to fail closed without Execute.
func breakerOpen() *mockresilience.MockBreaker {
	breaker := &mockresilience.MockBreaker{}
	breaker.On("State").Return(kernelres.StateOpen)

	return breaker
}

// egressWithError scripts every Wait call with the same admission failure.
func egressWithError(err error) *mockegress.MockWaiter {
	egress := &mockegress.MockWaiter{}
	egress.On("Wait", mock.Anything, mock.Anything, mock.Anything).Return(err)

	return egress
}

// requestFrom returns the request captured by the last Do call.
func requestFrom(doer *mockhttpclient.MockDoer) *http.Request {
	if len(doer.Calls) == 0 {
		return nil
	}

	req, _ := doer.Calls[len(doer.Calls)-1].Arguments.Get(0).(*http.Request)

	return req
}

// lastEgressKey returns provider/operation from the last Wait call, or "" when
// no permit was requested.
func lastEgressKey(waiter *mockegress.MockWaiter) string {
	for _, call := range slices.Backward(waiter.Calls) {
		if call.Method != "Wait" {
			continue
		}

		provider, _ := call.Arguments.Get(1).(string)
		operation, _ := call.Arguments.Get(2).(string)

		return provider + "/" + operation
	}

	return ""
}

// lastSetTTL returns the physical TTL passed to the last Set call, or zero when
// the cache was not written.
func lastSetTTL(cache *mockfx.MockRateCache) time.Duration {
	for _, call := range slices.Backward(cache.Calls) {
		if call.Method != "Set" {
			continue
		}

		ttl, _ := call.Arguments.Get(3).(time.Duration)

		return ttl
	}

	return 0
}
