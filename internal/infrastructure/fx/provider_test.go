package fx

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/coalesce"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	mockegress "github.com/kadekutama/go-template/test/mock/egress"
	mockfx "github.com/kadekutama/go-template/test/mock/fx"
	mockhttpclient "github.com/kadekutama/go-template/test/mock/httpclient"
	mockresilience "github.com/kadekutama/go-template/test/mock/resilience"
)

func TestProviderGetRate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ctx                  context.Context
		params               Params
		base                 valueobject.AssetCode
		quote                valueobject.AssetCode
		at                   time.Time
		expectedRate         valueobject.FxRate
		expectedError        error
		expectedEgressCalls  int
		expectedEgressKey    string
		expectedHTTPCalls    int
		expectedSetCalls     int
		expectedSetTTL       time.Duration
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name: "fresh cache hit performs no provider work",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(freshCachedRate(), nil)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         freshCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "zero valuation instant resolves to clock now",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(freshCachedRate(), nil)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   time.Time{},
			expectedRate:         freshCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "future valuation instant is clamped for freshness",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(freshCachedRate(), nil)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow().Add(time.Hour),
			expectedRate:         freshCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "expired cache refreshes and rewrites physical retention",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     1,
			expectedSetTTL:       24 * time.Hour,
			expectedBreakerCalls: 1,
		},
		{
			name:                 "empty cache fetches and caches",
			ctx:                  context.Background(),
			params:               testParams(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     1,
			expectedSetTTL:       24 * time.Hour,
			expectedBreakerCalls: 1,
		},
		{
			name: "cache read failure is treated as a miss",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(valueobject.FxRate{}, errCacheReadFailed)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     1,
			expectedSetTTL:       24 * time.Hour,
			expectedBreakerCalls: 1,
		},
		{
			name: "invalid cached value is treated as a miss",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(valueobject.FxRate{}, nil)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     1,
			expectedSetTTL:       24 * time.Hour,
			expectedBreakerCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			doer := tc.params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := tc.params.Egress.(*mockegress.MockWaiter)
			cacheMock := tc.params.Cache.(*mockfx.MockRateCache)
			breakerMock := tc.params.Breaker.(*mockresilience.MockBreaker)

			rate, err := provider.GetRate(tc.ctx, tc.base, tc.quote, tc.at)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): the side effects below
			// are the only permitted additions because the FxRate/error tuple
			// cannot distinguish a metered attempt from an unmetered one, or a
			// cache hit from a refresh with the same value.
			assert.Equal(t, tc.expectedEgressCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedEgressKey, lastEgressKey(egressMock))
			assert.Equal(t, tc.expectedHTTPCalls, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedSetCalls, callCount(&cacheMock.Mock, "Set"))
			assert.Equal(t, tc.expectedSetTTL, lastSetTTL(cacheMock))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breakerMock.Mock, "Execute"))
		})
	}
}

func TestProviderFallback(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ctx                  context.Context
		params               Params
		base                 valueobject.AssetCode
		quote                valueobject.AssetCode
		at                   time.Time
		expectedRate         valueobject.FxRate
		expectedError        error
		expectedEgressCalls  int
		expectedEgressKey    string
		expectedHTTPCalls    int
		expectedSetCalls     int
		expectedSetTTL       time.Duration
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name: "stale cache survives provider 503",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithResponse(http.StatusServiceUnavailable, "")

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
		{
			name: "stale cache survives provider 401 without retry",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithResponse(http.StatusUnauthorized, "")

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "stale cache survives provider 400 without retry",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithResponse(http.StatusBadRequest, "")

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "stale cache survives retryable 429",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithResponse(http.StatusTooManyRequests, "")

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
		{
			name: "stale cache survives transport failure",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithError(errTransportFailed)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
		{
			name: "stale cache survives an open breaker with no permit or HTTP",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.Breaker = breakerOpen()

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "stale cache survives egress wait denial with no HTTP",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.Egress = egressWithError(entity.NewError(egress.CodeWaitExceeded, "quota denied"))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "stale cache survives egress store failure with no HTTP",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.Egress = egressWithError(entity.NewError(egress.CodeStoreUnavailable, "store down"))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			doer := tc.params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := tc.params.Egress.(*mockegress.MockWaiter)
			cacheMock := tc.params.Cache.(*mockfx.MockRateCache)
			breakerMock := tc.params.Breaker.(*mockresilience.MockBreaker)

			rate, err := provider.GetRate(tc.ctx, tc.base, tc.quote, tc.at)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): the side effects below
			// are the only permitted additions because the FxRate/error tuple
			// cannot distinguish a metered attempt from an unmetered one, or a
			// cache hit from a refresh with the same value.
			assert.Equal(t, tc.expectedEgressCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedEgressKey, lastEgressKey(egressMock))
			assert.Equal(t, tc.expectedHTTPCalls, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedSetCalls, callCount(&cacheMock.Mock, "Set"))
			assert.Equal(t, tc.expectedSetTTL, lastSetTTL(cacheMock))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breakerMock.Mock, "Execute"))
		})
	}
}

func TestProviderUnavailablePath(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ctx                  context.Context
		params               Params
		base                 valueobject.AssetCode
		quote                valueobject.AssetCode
		at                   time.Time
		expectedRate         valueobject.FxRate
		expectedError        error
		expectedEgressCalls  int
		expectedEgressKey    string
		expectedHTTPCalls    int
		expectedSetCalls     int
		expectedSetTTL       time.Duration
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name: "empty cache and open breaker fails closed without permit",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Breaker = breakerOpen()

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderUnavailable, msgBreakerOpen),
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "empty cache and egress store failure fails closed",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Egress = egressWithError(entity.NewError(egress.CodeStoreUnavailable, "store down"))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeEgressUnavailable, "fx: provider egress admission failed (EGRESS_STORE_UNAVAILABLE)"),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "empty cache and transport failure is unavailable",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithError(errTransportFailed)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderUnavailable, msgRequestFailed),
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
		{
			name: "configured fetch timeout bounds the refresh loop",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.FetchTimeout = 10 * time.Millisecond
				params.HTTPClient = doerBlockedUntilContext()

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderUnavailable, msgFetchTimedOut),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "empty cache and provider 500 is unavailable after three attempts",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusInternalServerError, "")

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderUnavailable, msgProviderUnavailable),
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
		{
			name: "provider rejection fails closed without cache",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusOK, string(liveBody(false, "USD", "USDEUR", "0.92", testNow().Unix())))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderRejected, msgProviderRejected),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "malformed payload fails closed without retry",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusOK, `{"success":true,`)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeInvalidResponse, "fx: provider response does not parse"),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "stale cache survives a malformed payload",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.Cache = cacheWith(staleCachedRate(), nil)
				params.HTTPClient = doerWithResponse(http.StatusOK, `{"success":true,`)

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         staleCachedRate(),
			expectedError:        nil,
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			doer := tc.params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := tc.params.Egress.(*mockegress.MockWaiter)
			cacheMock := tc.params.Cache.(*mockfx.MockRateCache)
			breakerMock := tc.params.Breaker.(*mockresilience.MockBreaker)

			rate, err := provider.GetRate(tc.ctx, tc.base, tc.quote, tc.at)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): the side effects below
			// are the only permitted additions because the FxRate/error tuple
			// cannot distinguish a metered attempt from an unmetered one, or a
			// cache hit from a refresh with the same value.
			assert.Equal(t, tc.expectedEgressCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedEgressKey, lastEgressKey(egressMock))
			assert.Equal(t, tc.expectedHTTPCalls, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedSetCalls, callCount(&cacheMock.Mock, "Set"))
			assert.Equal(t, tc.expectedSetTTL, lastSetTTL(cacheMock))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breakerMock.Mock, "Execute"))
		})
	}
}

func TestProviderPayloadRejection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ctx                  context.Context
		params               Params
		base                 valueobject.AssetCode
		quote                valueobject.AssetCode
		at                   time.Time
		expectedRate         valueobject.FxRate
		expectedError        error
		expectedEgressCalls  int
		expectedEgressKey    string
		expectedHTTPCalls    int
		expectedSetCalls     int
		expectedSetTTL       time.Duration
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name: "missing quote fails closed",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusOK, string(liveBody(true, "USD", "USDGBP", "0.92", testNow().Unix())))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeInvalidResponse, "fx: provider response is missing the requested quote"),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "stale vendor timestamp fails closed",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusOK, string(liveBody(true, "USD", "USDEUR", "0.92", testNow().Add(-49*time.Hour).Unix())))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeInvalidResponse, "fx: provider quote timestamp is too old"),
			expectedEgressCalls:  1,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    1,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 1,
		},
		{
			name: "over-limit body is unavailable",
			ctx:  context.Background(),
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithResponse(http.StatusOK, strings.Repeat("a", int(params.MaxResponseBytes)+1))

				return params
			}(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodeProviderUnavailable, msgResponseReadFailed),
			expectedEgressCalls:  3,
			expectedEgressKey:    "fx/rates",
			expectedHTTPCalls:    3,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			doer := tc.params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := tc.params.Egress.(*mockegress.MockWaiter)
			cacheMock := tc.params.Cache.(*mockfx.MockRateCache)
			breakerMock := tc.params.Breaker.(*mockresilience.MockBreaker)

			rate, err := provider.GetRate(tc.ctx, tc.base, tc.quote, tc.at)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): the side effects below
			// are the only permitted additions because the FxRate/error tuple
			// cannot distinguish a metered attempt from an unmetered one, or a
			// cache hit from a refresh with the same value.
			assert.Equal(t, tc.expectedEgressCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedEgressKey, lastEgressKey(egressMock))
			assert.Equal(t, tc.expectedHTTPCalls, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedSetCalls, callCount(&cacheMock.Mock, "Set"))
			assert.Equal(t, tc.expectedSetTTL, lastSetTTL(cacheMock))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breakerMock.Mock, "Execute"))
		})
	}
}

func TestProviderInputRejection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                 string
		ctx                  context.Context
		params               Params
		base                 valueobject.AssetCode
		quote                valueobject.AssetCode
		at                   time.Time
		expectedRate         valueobject.FxRate
		expectedError        error
		expectedEgressCalls  int
		expectedEgressKey    string
		expectedHTTPCalls    int
		expectedSetCalls     int
		expectedSetTTL       time.Duration
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name:                 "equal pair rejected before cache or egress",
			ctx:                  context.Background(),
			params:               testParams(),
			base:                 "USD",
			quote:                "USD",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes"),
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name:                 "empty base rejected before cache or egress",
			ctx:                  context.Background(),
			params:               testParams(),
			base:                 "",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes"),
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name:                 "whitespace-only quote rejected before cache or egress",
			ctx:                  context.Background(),
			params:               testParams(),
			base:                 "USD",
			quote:                "   ",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError(CodePairInvalid, "fx rate pair requires distinct base and quote codes"),
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name:                 "control character code rejected before cache or egress",
			ctx:                  context.Background(),
			params:               testParams(),
			base:                 "US\nD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain control characters"),
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
		{
			name: "canceled context propagates without provider work",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			params:               testParams(),
			base:                 "USD",
			quote:                "EUR",
			at:                   testNow(),
			expectedRate:         valueobject.FxRate{},
			expectedError:        context.Canceled,
			expectedEgressCalls:  0,
			expectedEgressKey:    "",
			expectedHTTPCalls:    0,
			expectedSetCalls:     0,
			expectedSetTTL:       0,
			expectedBreakerCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			doer := tc.params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := tc.params.Egress.(*mockegress.MockWaiter)
			cacheMock := tc.params.Cache.(*mockfx.MockRateCache)
			breakerMock := tc.params.Breaker.(*mockresilience.MockBreaker)

			rate, err := provider.GetRate(tc.ctx, tc.base, tc.quote, tc.at)
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): the side effects below
			// are the only permitted additions because the FxRate/error tuple
			// cannot distinguish a metered attempt from an unmetered one, or a
			// cache hit from a refresh with the same value.
			assert.Equal(t, tc.expectedEgressCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedEgressKey, lastEgressKey(egressMock))
			assert.Equal(t, tc.expectedHTTPCalls, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedSetCalls, callCount(&cacheMock.Mock, "Set"))
			assert.Equal(t, tc.expectedSetTTL, lastSetTTL(cacheMock))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breakerMock.Mock, "Execute"))
		})
	}
}

func TestProviderCoalescing(t *testing.T) {
	t.Parallel()

	baseNow := testNow()

	type testCase struct {
		name            string
		firstBase       valueobject.AssetCode
		firstQuote      valueobject.AssetCode
		secondBase      valueobject.AssetCode
		secondQuote     valueobject.AssetCode
		expectedFirst   valueobject.FxRate
		expectedSecond  valueobject.FxRate
		expectedFetches int
	}

	testCases := []testCase{
		{
			name:            "identical concurrent misses share one fetch",
			firstBase:       "USD",
			firstQuote:      "EUR",
			secondBase:      "USD",
			secondQuote:     "EUR",
			expectedFirst:   expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedSecond:  expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedFetches: 1,
		},
		{
			name:            "distinct pairs do not share in-flight work",
			firstBase:       "USD",
			firstQuote:      "EUR",
			secondBase:      "USD",
			secondQuote:     "GBP",
			expectedFirst:   expectedLiveRate(baseNow, "EUR", 23, 25),
			expectedSecond:  expectedLiveRate(baseNow, "GBP", 23, 25),
			expectedFetches: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			coalescer, err := coalesce.NewCoalescer(coalesce.Params{
				BaseContext:   context.Background(),
				SharedTimeout: 2 * time.Second,
			})
			require.NoError(t, err)

			params := testParams()
			params.Coalescer = coalescer
			params.Cache = cacheWith(valueobject.FxRate{}, hybrid.ErrCacheMiss)
			params.HTTPClient = doerPerRequest(func(req *http.Request) (*http.Response, error) {
				time.Sleep(20 * time.Millisecond)

				quote := valueobject.AssetCode(req.URL.Query().Get("currencies"))

				return testResponse(
					req,
					http.StatusOK,
					string(liveBody(true, "USD", "USD"+string(quote), "0.92", baseNow.Unix())),
				), nil
			})

			provider, err := NewProvider(params)
			require.NoError(t, err)

			doer := params.HTTPClient.(*mockhttpclient.MockDoer)
			egressMock := params.Egress.(*mockegress.MockWaiter)
			cacheMock := params.Cache.(*mockfx.MockRateCache)

			results := make([]valueobject.FxRate, 2)
			errs := make([]error, 2)

			var wg sync.WaitGroup

			wg.Add(2)

			for index := range 2 {
				go func(index int) {
					defer wg.Done()

					if index == 0 {
						results[index], errs[index] = provider.GetRate(context.Background(), tc.firstBase, tc.firstQuote, baseNow)

						return
					}

					results[index], errs[index] = provider.GetRate(context.Background(), tc.secondBase, tc.secondQuote, baseNow)
				}(index)
			}

			wg.Wait()

			assert.Equal(t, tc.expectedFirst, results[0])
			assert.NoError(t, errs[0])
			assert.Equal(t, tc.expectedSecond, results[1])
			assert.NoError(t, errs[1])
			assert.Equal(t, tc.expectedFetches, callCount(&doer.Mock, "Do"))
			assert.Equal(t, tc.expectedFetches, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedFetches, callCount(&cacheMock.Mock, "Set"))
		})
	}
}

func TestProviderRequestShape(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		baseURL       string
		livePath      string
		base          valueobject.AssetCode
		quote         valueobject.AssetCode
		expectedPath  string
		expectedQuery url.Values
		expectedRate  valueobject.FxRate
		expectedError error
	}

	testCases := []testCase{
		{
			name:         "live path carries the credential and pair",
			baseURL:      "https://fx.example.test",
			livePath:     "/live",
			base:         "USD",
			quote:        "EUR",
			expectedPath: "/live",
			expectedQuery: url.Values{
				"access_key": {testAPIKey},
				"source":     {"USD"},
				"currencies": {"EUR"},
			},
			expectedRate:  expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:         "trailing slash and outer whitespace are canonicalized",
			baseURL:      "https://fx.example.test/",
			livePath:     "/live",
			base:         " USD ",
			quote:        " EUR ",
			expectedPath: "/live",
			expectedQuery: url.Values{
				"access_key": {testAPIKey},
				"source":     {"USD"},
				"currencies": {"EUR"},
			},
			expectedRate:  expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError: nil,
		},
		{
			name:         "configured live path is honored",
			baseURL:      "https://fx.example.test",
			livePath:     "/v1/rates/live",
			base:         "USD",
			quote:        "EUR",
			expectedPath: "/v1/rates/live",
			expectedQuery: url.Values{
				"access_key": {testAPIKey},
				"source":     {"USD"},
				"currencies": {"EUR"},
			},
			expectedRate:  expectedLiveRate(testNow(), "EUR", 23, 25),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params := testParams()
			params.BaseURL = tc.baseURL
			params.LivePath = tc.livePath
			provider, err := NewProvider(params)
			require.NoError(t, err)

			doer := params.HTTPClient.(*mockhttpclient.MockDoer)

			rate, err := provider.GetRate(context.Background(), tc.base, tc.quote, testNow())
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): request shape (R02/S02)
			// is not part of the returned tuple.
			req := requestFrom(doer)
			require.NotNil(t, req)
			assert.Equal(t, tc.expectedPath, req.URL.Path)
			assert.Equal(t, tc.expectedQuery, req.URL.Query())
		})
	}
}

func TestProviderErrorRedaction(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        Params
		expectedRate  valueobject.FxRate
		expectedError error
		requiredText  string
		forbiddenText []string
	}

	testCases := []testCase{
		{
			name: "transport failure redacts the key and query",
			params: func() Params {
				params := testParams()
				params.HTTPClient = doerWithError(errTransportFailed)

				return params
			}(),
			expectedRate:  valueobject.FxRate{},
			expectedError: entity.NewError(CodeProviderUnavailable, msgRequestFailed),
			requiredText:  "[REDACTED]",
			forbiddenText: []string{testAPIKey, "https://fx.example.test"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			require.NoError(t, err)

			rate, err := provider.GetRate(context.Background(), "USD", "EUR", testNow())
			assert.Equal(t, tc.expectedRate, rate)
			assert.Equal(t, tc.expectedError, err)

			// Test Convention Exception (2026-10-28): redaction (R09/S21)
			// inspects the returned error text itself.
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.requiredText)

			for _, forbidden := range tc.forbiddenText {
				assert.NotContains(t, err.Error(), forbidden)
			}
		})
	}
}
