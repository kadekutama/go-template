// Package fx is the E10-T02 G4 slice: cache-aside freshness, physical stale
// retention, visible stale fallback, a real gobreaker, single-flight
// coalescing, and one egress permit per HTTP attempt, all over Testcontainers
// Valkey. Suites skip without Docker.
package fx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/local"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	fxadapter "github.com/kadekutama/go-template/internal/infrastructure/fx"
	infraresilience "github.com/kadekutama/go-template/internal/infrastructure/resilience"
	kernelres "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	testcontainers "github.com/kadekutama/go-template/test/testcontainers"
)

const (
	testSource = "exchangerate.host"
	testAPIKey = "integration-secret-key"
	quoteKey   = "fx:USD:EUR"
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

// newMockClock returns a generated clock mock whose current instant can be
// advanced atomically between test phases.
func newMockClock(now time.Time) (*mockapplication.MockClock, func(time.Time)) {
	var nanos atomic.Int64

	nanos.Store(now.UnixNano())

	clock := &mockapplication.MockClock{}
	clock.On("Now").Return(func() time.Time {
		return time.Unix(0, nanos.Load()).UTC()
	})

	return clock, func(next time.Time) {
		nanos.Store(next.UnixNano())
	}
}

type harness struct {
	provider *fxadapter.Provider
	typed    *hybrid.TypedCache[valueobject.FxRate]
	clock    *mockapplication.MockClock
}

type harnessParams struct {
	serverURL        string
	clock            *mockapplication.MockClock
	budget           int64
	maxWait          time.Duration
	failureThreshold uint32
}

func errorCodeOf(err error) string {
	if err == nil {
		return ""
	}

	coded, ok := errors.AsType[*entity.Error](err)
	if ok {
		return coded.Code
	}

	return err.Error()
}

func liveBody(source, quoteKey, quoteValue string, timestamp int64) []byte {
	return []byte(`{"success":true,"source":"` + source + `","quotes":{"` + quoteKey +
		`":` + quoteValue + `},"timestamp":` + strconv.FormatInt(timestamp, 10) + `}`)
}

func newHarness(t *testing.T, params harnessParams) *harness {
	t.Helper()

	handle, err := testcontainers.StartValkey(t)
	require.NoError(t, err)

	l1, err := local.NewOtterCache(local.OtterParams{MaximumWeight: 1 << 20, DefaultTTL: time.Hour})
	require.NoError(t, err)
	t.Cleanup(func() { _ = l1.Close() })

	l2, err := valkey.NewValkeyClient(valkey.Params{
		Addr:         handle.Addr(),
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = l2.Close() })

	engine, err := hybrid.New(hybrid.Params{
		L1:            l1,
		L2:            l2,
		L1PopulateTTL: time.Minute,
		Codec:         hybrid.JSONCodec{},
	})
	require.NoError(t, err)

	rateLimiter, err := valkey.NewRateLimiter(valkey.LimiterParams{Client: l2})
	require.NoError(t, err)

	limiter, err := egress.NewEgressLimiter(egress.Params{
		Limiter: rateLimiter,
		Policies: []egress.Policy{
			{
				Provider:  "fx",
				Operation: "rates",
				Budget:    params.budget,
				Window:    time.Hour,
				MaxWait:   params.maxWait,
			},
		},
	})
	require.NoError(t, err)

	breaker := infraresilience.New("fx", infraresilience.Settings{
		FailureThreshold: params.failureThreshold,
		ProbeTimeout:     time.Hour,
	})

	coalescer, err := coalesce.NewCoalescer(coalesce.Params{
		BaseContext:   context.Background(),
		SharedTimeout: 5 * time.Second,
	})
	require.NoError(t, err)

	typed := hybrid.Typed[valueobject.FxRate](engine)

	provider, err := fxadapter.NewProvider(fxadapter.Params{
		BaseURL:          params.serverURL,
		APIKey:           testAPIKey,
		Source:           testSource,
		LivePath:         "/live",
		HTTPClient:       http.DefaultClient,
		Cache:            typed,
		Egress:           limiter,
		Breaker:          breaker,
		Coalescer:        coalescer,
		Clock:            params.clock,
		RetryPolicy:      testRetryPolicy(),
		FetchTimeout:     5 * time.Second,
		MaxResponseBytes: 1 << 20,
		MaxFutureSkew:    5 * time.Minute,
		MaxDecimalLen:    40,
		TTL:              time.Minute,
		StaleRetention:   24 * time.Hour,
		MaxQuoteAge:      48 * time.Hour,
	})
	require.NoError(t, err)

	return &harness{provider: provider, typed: typed, clock: params.clock}
}

// newServer quotes exactly USD→EUR. The status function lets a test flip the
// provider between healthy and failing at runtime.
func newServer(t *testing.T, status func() int, delay time.Duration) (*httptest.Server, *atomic.Int64) {
	t.Helper()

	var hits atomic.Int64

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hits.Add(1)

		if delay > 0 {
			time.Sleep(delay)
		}

		code := status()
		if code != http.StatusOK {
			writer.WriteHeader(code)

			return
		}

		body := liveBody("USD", "USDEUR", "0.92", testNow().Unix())

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write(body)
	}))
	t.Cleanup(server.Close)

	return server, &hits
}

func TestProviderCacheHitSkipsProvider(t *testing.T) {
	t.Parallel()

	clock, _ := newMockClock(testNow())
	server, hits := newServer(t, func() int { return http.StatusOK }, 0)
	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           100,
		maxWait:          time.Second,
		failureThreshold: 100,
	})

	ctx := context.Background()

	first, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	require.NoError(t, err)
	require.Equal(t, int64(1), hits.Load())

	second, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	require.NoError(t, err)

	assert.Equal(t, first, second)
	assert.Equal(t, int64(1), hits.Load())
	assert.False(t, second.IsStale(clock.Now()))
}

func TestProviderStaleFallbackKeepsPhysicalRetention(t *testing.T) {
	t.Parallel()

	clock, setClock := newMockClock(testNow())

	var failing atomic.Bool

	server, hits := newServer(t, func() int {
		if failing.Load() {
			return http.StatusServiceUnavailable
		}

		return http.StatusOK
	}, 0)

	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           100,
		maxWait:          time.Second,
		failureThreshold: 100,
	})

	ctx := context.Background()

	fresh, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	require.NoError(t, err)

	// Logical TTL is one minute; physical retention is 24h. Advancing only
	// the adapter clock makes the cached entry logically stale while the
	// Valkey copy remains.
	setClock(testNow().Add(2 * time.Minute))
	failing.Store(true)

	stale, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	require.NoError(t, err)

	assert.Equal(t, fresh, stale)
	assert.True(t, stale.IsStale(clock.Now()))
	assert.Equal(t, int64(1+testRetryPolicy().MaxAttempts), hits.Load())

	retained, err := h.typed.Get(ctx, quoteKey)
	require.NoError(t, err)
	assert.Equal(t, fresh, retained)
}

func TestProviderBreakerOpenStopsHTTP(t *testing.T) {
	t.Parallel()

	clock, _ := newMockClock(testNow())
	server, hits := newServer(t, func() int { return http.StatusServiceUnavailable }, 0)
	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           100,
		maxWait:          time.Second,
		failureThreshold: 1,
	})

	ctx := context.Background()

	_, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	assert.Equal(t, fxadapter.CodeProviderUnavailable, errorCodeOf(err))

	// FailureThreshold 1 opens after the first 5xx; the retry loop
	// short-circuits before any further permit or HTTP attempt.
	assert.Equal(t, int64(1), hits.Load())

	_, err = h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	assert.Equal(t, fxadapter.CodeProviderUnavailable, errorCodeOf(err))
	assert.Equal(t, int64(1), hits.Load())
}

func TestProviderCoalescesIdenticalMisses(t *testing.T) {
	t.Parallel()

	clock, _ := newMockClock(testNow())
	server, hits := newServer(t, func() int { return http.StatusOK }, 50*time.Millisecond)
	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           100,
		maxWait:          time.Second,
		failureThreshold: 100,
	})

	ctx := context.Background()

	rates := make([]valueobject.FxRate, 2)
	errs := make([]error, 2)

	var wg sync.WaitGroup

	wg.Add(2)

	for index := range 2 {
		go func(index int) {
			defer wg.Done()

			rates[index], errs[index] = h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
		}(index)
	}

	wg.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	assert.Equal(t, rates[0], rates[1])
	assert.Equal(t, int64(1), hits.Load())
}

func TestProviderSpendsOnePermitPerAttempt(t *testing.T) {
	t.Parallel()

	clock, _ := newMockClock(testNow())
	server, hits := newServer(t, func() int { return http.StatusServiceUnavailable }, 0)
	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           2,
		maxWait:          100 * time.Millisecond,
		failureThreshold: 100,
	})

	ctx := context.Background()

	_, err := h.provider.GetRate(ctx, "USD", "EUR", clock.Now())
	assert.Equal(t, fxadapter.CodeEgressUnavailable, errorCodeOf(err))

	// Budget 2 admits exactly two of the three retry attempts; the third
	// fails admission before reaching the provider.
	assert.Equal(t, int64(2), hits.Load())
}

// TestProviderUnknownPairIsUnavailable proves a structurally valid request for
// a pair the provider does not quote fails closed rather than caching a guess.
func TestProviderUnknownPairIsUnavailable(t *testing.T) {
	t.Parallel()

	clock, _ := newMockClock(testNow())
	server, _ := newServer(t, func() int { return http.StatusOK }, 0)
	h := newHarness(t, harnessParams{
		serverURL:        server.URL,
		clock:            clock,
		budget:           100,
		maxWait:          time.Second,
		failureThreshold: 100,
	})

	ctx := context.Background()

	_, err := h.provider.GetRate(ctx, "USD", "GBP", clock.Now())
	assert.Equal(t, fxadapter.CodeInvalidResponse, errorCodeOf(err))

	_, err = h.typed.Get(ctx, "fx:USD:GBP")
	assert.ErrorIs(t, err, hybrid.ErrCacheMiss)
}
