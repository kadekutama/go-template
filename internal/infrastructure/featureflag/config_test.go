package featureflag_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/featureflag"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
)

func testRetryPolicy() kernelresilience.RetryPolicy {
	return kernelresilience.RetryPolicy{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     5 * time.Millisecond,
		Multiplier:     2.0,
	}
}

func testParams() featureflag.Params {
	return featureflag.Params{
		UnleashURL:        "http://unleash:4242/api",
		AppName:           "ledger-test",
		RefreshInterval:   10 * time.Second,
		InitTimeout:       5 * time.Second,
		CacheTTL:          time.Hour,
		ProbeMaxBodyBytes: 1024,
		Transport:         newMockRoundTripper(http.StatusOK, nil),
		RetryPolicy:       testRetryPolicy(),
		Egress:            newMockEgress(nil),
		Breaker:           newMockBreaker(nil),
		HTTP:              newMockDoer(http.StatusOK, nil),
		Clock:             testClock(time.Now()),
		Evaluator:         newMockEvaluator(false, nil, nil, nil),
	}
}

func TestParamsValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            featureflag.Params
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "valid params accepted",
			params:            testParams(),
			expectedErrorCode: "",
		},
		{
			name: "missing unleash url rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.UnleashURL = ""

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "malformed unleash url rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.UnleashURL = "not-a-url"

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "missing app name rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.AppName = ""

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "zero refresh interval rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.RefreshInterval = 0

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "negative refresh interval rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.RefreshInterval = -time.Second

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "zero cache ttl rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.CacheTTL = 0

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "negative cache ttl rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.CacheTTL = -time.Second

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "zero probe max body rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.ProbeMaxBodyBytes = 0

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "missing transport rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Transport = nil

				return params
			}(),
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero init timeout rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.InitTimeout = 0

				return params
			}(),
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "missing egress rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Egress = nil

				return params
			}(),
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing breaker rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Breaker = nil

				return params
			}(),
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing http rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.HTTP = nil

				return params
			}(),
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing clock rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Clock = nil

				return params
			}(),
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero retry attempts rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.RetryPolicy.MaxAttempts = 0

				return params
			}(),
			expectedErrorCode: "resilience: max attempts must be positive",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.params.Validate()
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestLoadEgressPolicies(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		data              []byte
		expectedResult    []egress.Policy
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "valid unleash policy accepted",
			data: []byte("policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 100\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n"),
			expectedResult: []egress.Policy{
				{
					Provider:  "unleash",
					Operation: "flags",
					Budget:    100,
					Window:    time.Minute,
					MaxWait:   5 * time.Second,
				},
			},
			expectedErrorCode: "",
		},
		{
			name:              "empty document rejected",
			data:              []byte("policies: []\n"),
			expectedResult:    nil,
			expectedErrorCode: "EGRESS_INVALID_POLICY",
		},
		{
			name: "wrong provider rejected",
			data: []byte("policies:\n" +
				"  - provider: fx\n" +
				"    operation: flags\n" +
				"    budget: 100\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n"),
			expectedResult:    nil,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "wrong operation rejected",
			data: []byte("policies:\n" +
				"  - provider: unleash\n" +
				"    operation: rates\n" +
				"    budget: 100\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n"),
			expectedResult:    nil,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero budget rejected",
			data: []byte("policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 0\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n"),
			expectedResult:    nil,
			expectedErrorCode: "EGRESS_INVALID_POLICY",
		},
		{
			name: "bad duration rejected",
			data: []byte("policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 10\n" +
				"    window: soon\n" +
				"    max_wait: 5s\n"),
			expectedResult:    nil,
			expectedErrorCode: "EGRESS_INVALID_POLICY",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			policies, err := featureflag.LoadEgressPolicies(tc.data)
			assert.Equal(t, tc.expectedResult, policies)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestNewFallbackValidation(t *testing.T) {
	t.Parallel()

	now := time.Now()

	type testCase struct {
		name              string
		params            featureflag.FallbackParams
		expectedNilResult bool
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "valid fallback accepted",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
				CacheTTL:        time.Hour,
			},
			expectedNilResult: false,
			expectedErrorCode: "",
		},
		{
			name: "nil defaults accepted",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
				Defaults:        nil,
				CacheTTL:        time.Hour,
			},
			expectedNilResult: false,
			expectedErrorCode: "",
		},
		{
			name: "custom cache ttl accepted",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
				CacheTTL:        30 * time.Minute,
			},
			expectedNilResult: false,
			expectedErrorCode: "",
		},
		{
			name: "missing clock rejected",
			params: featureflag.FallbackParams{
				RefreshInterval: 10 * time.Second,
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero refresh rejected",
			params: featureflag.FallbackParams{
				Clock: testClock(now),
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "negative refresh rejected",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: -time.Second,
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "negative cache ttl rejected",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
				CacheTTL:        -time.Second,
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero cache ttl rejected",
			params: featureflag.FallbackParams{
				Clock:           testClock(now),
				RefreshInterval: 10 * time.Second,
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fallback, err := featureflag.NewFallback(tc.params)
			assert.Equal(t, tc.expectedNilResult, fallback == nil)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestNewGatedTransportValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		egressWaiter      featureflag.EgressWaiter
		breaker           kernelresilience.Breaker
		base              http.RoundTripper
		expectedNilResult bool
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "valid transport accepted",
			egressWaiter:      newMockEgress(nil),
			breaker:           newMockBreaker(nil),
			base:              newMockRoundTripper(http.StatusOK, nil),
			expectedNilResult: false,
			expectedErrorCode: "",
		},
		{
			name:              "missing base rejected",
			egressWaiter:      newMockEgress(nil),
			breaker:           newMockBreaker(nil),
			base:              nil,
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name:              "missing egress rejected",
			egressWaiter:      nil,
			breaker:           newMockBreaker(nil),
			base:              nil,
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name:              "missing breaker rejected",
			egressWaiter:      newMockEgress(nil),
			breaker:           nil,
			base:              nil,
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			transport, err := featureflag.NewGatedTransport(tc.egressWaiter, tc.breaker, tc.base, nil)
			assert.Equal(t, tc.expectedNilResult, transport == nil)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestNewUnleashEvaluatorValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            featureflag.UnleashEvaluatorParams
		expectedNilResult bool
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "missing url rejected",
			params: featureflag.UnleashEvaluatorParams{
				AppName:         "ledger-test",
				RefreshInterval: 10 * time.Second,
				InitTimeout:     5 * time.Second,
				HTTPClient:      &http.Client{},
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing app name rejected",
			params: featureflag.UnleashEvaluatorParams{
				UnleashURL:      "http://unleash:4242/api",
				RefreshInterval: 10 * time.Second,
				InitTimeout:     5 * time.Second,
				HTTPClient:      &http.Client{},
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero refresh rejected",
			params: featureflag.UnleashEvaluatorParams{
				UnleashURL:  "http://unleash:4242/api",
				AppName:     "ledger-test",
				InitTimeout: 5 * time.Second,
				HTTPClient:  &http.Client{},
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "zero init timeout rejected",
			params: featureflag.UnleashEvaluatorParams{
				UnleashURL:      "http://unleash:4242/api",
				AppName:         "ledger-test",
				RefreshInterval: 10 * time.Second,
				HTTPClient:      &http.Client{},
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing http client rejected",
			params: featureflag.UnleashEvaluatorParams{
				UnleashURL:      "http://unleash:4242/api",
				AppName:         "ledger-test",
				RefreshInterval: 10 * time.Second,
				InitTimeout:     5 * time.Second,
			},
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			evaluator, err := featureflag.NewUnleashEvaluator(tc.params)
			assert.Equal(t, tc.expectedNilResult, evaluator == nil)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}

func TestNewClientValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            featureflag.Params
		expectedNilResult bool
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name:              "valid client with stub evaluator accepted",
			params:            testParams(),
			expectedNilResult: false,
			expectedErrorCode: "",
		},
		{
			name: "missing url rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.UnleashURL = ""

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: "VALIDATION",
		},
		{
			name: "missing egress rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Egress = nil

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing breaker rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Breaker = nil

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing http rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.HTTP = nil

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "missing clock rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.Clock = nil

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: featureflag.CodeInvalidConfig,
		},
		{
			name: "negative cache ttl rejected",
			params: func() featureflag.Params {
				params := testParams()
				params.CacheTTL = -time.Second

				return params
			}(),
			expectedNilResult: true,
			expectedErrorCode: "VALIDATION",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			client, err := featureflag.NewClient(tc.params)
			assert.Equal(t, tc.expectedNilResult, client == nil)
			assert.Equal(t, tc.expectedErrorCode, errorCodeOf(err))
		})
	}
}
