package featureflag_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	mockfeatureflag "github.com/kadekutama/go-template/test/mock/featureflag"
	mockhttpclient "github.com/kadekutama/go-template/test/mock/httpclient"
	mockresilience "github.com/kadekutama/go-template/test/mock/resilience"
)

func testClientParams(
	clock appport.Clock,
	egressWaiter *mockfeatureflag.MockEgressWaiter,
	breaker *mockresilience.MockBreaker,
	doer *mockhttpclient.MockDoer,
	evaluator *mockfeatureflag.MockEvaluator,
) featureflag.Params {
	return featureflag.Params{
		UnleashURL:        "http://unleash:4242/api",
		AppName:           "ledger-test",
		RefreshInterval:   10 * time.Second,
		CacheTTL:          time.Hour,
		InitTimeout:       5 * time.Second,
		ProbeMaxBodyBytes: 1024,
		Transport:         newMockRoundTripper(http.StatusOK, nil),
		RetryPolicy: kernelresilience.RetryPolicy{
			MaxAttempts:    3,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     5 * time.Millisecond,
			Multiplier:     2.0,
		},
		Egress:    egressWaiter,
		Breaker:   breaker,
		HTTP:      doer,
		Clock:     clock,
		Evaluator: evaluator,
	}
}

// seedFallback builds a production fallback with one recorded last-known
// value. Observe is void, so seeding performs no assertion on any function
// under test; the measured calls below carry the assertions.
func seedFallback(t *testing.T, clock appport.Clock, key string, value bool) *featureflag.Fallback {
	t.Helper()

	fallback, err := featureflag.NewFallback(featureflag.FallbackParams{
		Clock:           clock,
		RefreshInterval: 10 * time.Second,
		CacheTTL:        time.Hour,
	})
	require.NoError(t, err)
	fallback.Observe(context.Background(), key, value)

	return fallback
}

func TestClientBooleanWithDetails(t *testing.T) {
	t.Parallel()

	baseCtx := appport.FlagContext{
		UserID:   "user-123",
		TenantID: "tenant-456",
		Attributes: map[string]string{
			"payment_method": "card",
		},
	}

	type testCase struct {
		name                 string
		ctx                  context.Context
		key                  string
		def                  bool
		evalCtx              appport.FlagContext
		evalValue            bool
		evalErr              error
		evalScript           []mockEvalStep
		defaults             map[string]bool
		seedKey              string
		seedValue            bool
		advance              time.Duration
		breakerOpen          bool
		expectedResult       appport.FlagDetails
		expectedErrorCode    string
		expectedEvalCalls    int
		expectedBreakerCalls int
	}

	testCases := []testCase{
		{
			name:                 "provider success returns fresh",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			evalValue:            true,
			expectedResult:       appport.FlagDetails{Value: true},
			expectedEvalCalls:    1,
			expectedBreakerCalls: 1,
		},
		{
			name:                 "provider success false returns fresh false",
			ctx:                  context.Background(),
			key:                  "currency.EUR.enabled",
			def:                  true,
			evalCtx:              baseCtx,
			evalValue:            false,
			expectedResult:       appport.FlagDetails{Value: false},
			expectedEvalCalls:    1,
			expectedBreakerCalls: 1,
		},
		{
			name:                 "provider down without cache serves configured default",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			evalErr:              providerUnavailable(),
			defaults:             map[string]bool{"payMethod.card.enabled": true},
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true},
			expectedErrorCode:    featureflag.CodeProviderUnavailable,
			expectedEvalCalls:    3,
			expectedBreakerCalls: 3,
		},
		{
			name:                 "provider down without cache or default serves call default",
			ctx:                  context.Background(),
			key:                  "kill.switch",
			def:                  true,
			evalCtx:              baseCtx,
			evalErr:              providerUnavailable(),
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true},
			expectedErrorCode:    featureflag.CodeProviderUnavailable,
			expectedEvalCalls:    3,
			expectedBreakerCalls: 3,
		},
		{
			name:                 "seeded last known served on provider outage",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			evalErr:              providerUnavailable(),
			seedKey:              "payMethod.card.enabled",
			seedValue:            true,
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true},
			expectedErrorCode:    featureflag.CodeProviderUnavailable,
			expectedEvalCalls:    3,
			expectedBreakerCalls: 3,
		},
		{
			name:                 "seeded last known past bound served stale",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			evalErr:              providerUnavailable(),
			seedKey:              "payMethod.card.enabled",
			seedValue:            true,
			advance:              11 * time.Second,
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true, Stale: true},
			expectedErrorCode:    featureflag.CodeProviderUnavailable,
			expectedEvalCalls:    3,
			expectedBreakerCalls: 3,
		},
		{
			name:                 "breaker open fast-fails to seeded fallback",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			seedKey:              "payMethod.card.enabled",
			seedValue:            true,
			breakerOpen:          true,
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true},
			expectedErrorCode:    featureflag.CodeBreakerOpen,
			expectedEvalCalls:    0,
			expectedBreakerCalls: 1,
		},
		{
			name: "canceled context serves seeded fallback",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			seedKey:              "payMethod.card.enabled",
			seedValue:            true,
			expectedResult:       appport.FlagDetails{Value: true, Fallback: true},
			expectedErrorCode:    "CANCELED",
			expectedEvalCalls:    0,
			expectedBreakerCalls: 0,
		},
		{
			name:                 "retryable then success returns fresh",
			ctx:                  context.Background(),
			key:                  "payMethod.card.enabled",
			def:                  false,
			evalCtx:              baseCtx,
			evalScript:           []mockEvalStep{{value: false, err: providerUnavailable()}, {value: true}},
			expectedResult:       appport.FlagDetails{Value: true},
			expectedEvalCalls:    2,
			expectedBreakerCalls: 2,
		},
		{
			name: "permanent error does not retry",
			ctx:  context.Background(),
			key:  "payMethod.card.enabled",
			def:  false,
			evalCtx: func() appport.FlagContext {
				return baseCtx
			}(),
			evalScript: []mockEvalStep{
				{
					value: false,
					err: &kernelresilience.PermanentError{
						Err: providerUnavailable(),
					},
				},
			},
			expectedResult:       appport.FlagDetails{Value: false, Fallback: true},
			expectedErrorCode:    featureflag.CodeProviderUnavailable,
			expectedEvalCalls:    1,
			expectedBreakerCalls: 1,
		},
		{
			name: "empty key rejected",
			ctx:  context.Background(),
			key:  "",
			def:  false,
			evalCtx: func() appport.FlagContext {
				return baseCtx
			}(),
			expectedResult:       appport.FlagDetails{Value: false, Fallback: true},
			expectedErrorCode:    featureflag.CodeInvalidFlagKey,
			expectedEvalCalls:    0,
			expectedBreakerCalls: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Now()
			clock := testClock(now, now.Add(tc.advance))
			egressMock := newMockEgress(nil)

			breaker := newMockBreaker(nil)
			if tc.breakerOpen {
				breaker = newMockBreaker(breakerOpen())
			}

			doer := newMockDoer(http.StatusOK, nil)
			evaluator := newMockEvaluator(tc.evalValue, tc.evalErr, tc.evalScript, nil)

			params := testClientParams(clock, egressMock, breaker, doer, evaluator)
			params.Defaults = tc.defaults

			if tc.seedKey != "" {
				params.Fallback = seedFallback(t, clock, tc.seedKey, tc.seedValue)
			}

			client, err := featureflag.NewClient(params)
			require.NoError(t, err)

			actualResult, err := client.BooleanWithDetails(tc.ctx, tc.key, tc.def, tc.evalCtx)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))

			// Side-effect proof under the packet exception: retry and
			// fast-fail behavior cannot be distinguished from return values
			// alone.
			assert.Equal(t, tc.expectedEvalCalls, callCount(&evaluator.Mock, "Evaluate"))
			assert.Equal(t, tc.expectedBreakerCalls, callCount(&breaker.Mock, "Execute"))
		})
	}
}

func TestClientRefresh(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		egressErr         error
		breakerOpen       bool
		probeStatus       int
		probeErr          error
		cancelBefore      bool
		expectedErrorCode string
		expectedWaitCalls int
		expectedDoCalls   int
	}

	testCases := []testCase{
		{
			name:              "healthy probe succeeds with one permit and one request",
			probeStatus:       http.StatusOK,
			expectedWaitCalls: 1,
			expectedDoCalls:   1,
		},
		{
			name:              "limiter store failure fails closed without http request",
			egressErr:         entity.NewError(egress.CodeStoreUnavailable, "egress: limiter store unavailable"),
			probeStatus:       http.StatusOK,
			expectedErrorCode: egress.CodeStoreUnavailable,
			expectedWaitCalls: 1,
			expectedDoCalls:   0,
		},
		{
			name:              "missing policy fails closed without http request",
			egressErr:         entity.NewError(egress.CodeInvalidPolicy, "egress: no policy for provider operation"),
			probeStatus:       http.StatusOK,
			expectedErrorCode: egress.CodeInvalidPolicy,
			expectedWaitCalls: 1,
			expectedDoCalls:   0,
		},
		{
			name:              "breaker open fails closed without http request",
			breakerOpen:       true,
			probeStatus:       http.StatusOK,
			expectedErrorCode: featureflag.CodeBreakerOpen,
			expectedWaitCalls: 1,
			expectedDoCalls:   0,
		},
		{
			name:              "retryable probe status retries with permit per attempt",
			probeStatus:       http.StatusInternalServerError,
			expectedErrorCode: "featureflag: refresh attempts exhausted: featureflag: refresh probe failed: status 500",
			expectedWaitCalls: 3,
			expectedDoCalls:   3,
		},
		{
			name:              "canceled context performs no wait or request",
			cancelBefore:      true,
			probeStatus:       http.StatusOK,
			expectedErrorCode: "CANCELED",
			expectedWaitCalls: 0,
			expectedDoCalls:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			clock := testClock(time.Now())
			egressMock := newMockEgress(tc.egressErr)

			breaker := newMockBreaker(nil)
			if tc.breakerOpen {
				breaker = newMockBreaker(breakerOpen())
			}

			typedDoer := newMockDoer(tc.probeStatus, tc.probeErr)

			params := testClientParams(clock, egressMock, breaker, typedDoer, newMockEvaluator(false, nil, nil, nil))
			client, err := featureflag.NewClient(params)
			require.NoError(t, err)

			ctx := context.Background()

			if tc.cancelBefore {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			err = client.Refresh(ctx)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))

			// Side-effect proof under the packet exception: each attempt
			// reacquires a permit; limiter failures never reach the network.
			assert.Equal(t, tc.expectedWaitCalls, callCount(&egressMock.Mock, "Wait"))
			assert.Equal(t, tc.expectedDoCalls, callCount(&typedDoer.Mock, "Do"))
		})
	}
}

func TestClientHealth(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		healthErr         error
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "healthy provider reports healthy",
			healthErr:         nil,
			expectedErrorCode: "",
		},
		{
			name:              "unhealthy provider reports error",
			healthErr:         providerUnavailable(),
			expectedErrorCode: featureflag.CodeProviderUnavailable,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			params := testClientParams(
				testClock(time.Now()),
				newMockEgress(nil),
				newMockBreaker(nil),
				newMockDoer(http.StatusOK, nil),
				newMockEvaluator(false, nil, nil, tc.healthErr),
			)

			client, err := featureflag.NewClient(params)
			require.NoError(t, err)

			err = client.Health(context.Background())
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestClientClose(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            featureflag.Params
		closeCalls        int
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "close releases resources",
			params:            testClientParams(testClock(time.Now()), newMockEgress(nil), newMockBreaker(nil), newMockDoer(http.StatusOK, nil), newMockEvaluator(false, nil, nil, nil)),
			closeCalls:        1,
			expectedErrorCode: "",
		},
		{
			name: "close twice stays idempotent",
			params: func() featureflag.Params {
				return testClientParams(testClock(time.Now()), newMockEgress(nil), newMockBreaker(nil), newMockDoer(http.StatusOK, nil), newMockEvaluator(false, nil, nil, nil))
			}(),
			closeCalls:        2,
			expectedErrorCode: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := featureflag.NewClient(tc.params)
			require.NoError(t, err)

			for range tc.closeCalls {
				err = client.Close(context.Background())
				assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
			}
		})
	}
}
