package fx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	kernelres "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
)

func TestNewProviderRequiredSeams(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         Params
		expectedResult *Provider
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "missing http client rejected",
			params: func() Params {
				params := testParams()
				params.HTTPClient = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: http client is required"),
		},
		{
			name: "missing cache rejected",
			params: func() Params {
				params := testParams()
				params.Cache = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: rate cache is required"),
		},
		{
			name: "missing egress rejected",
			params: func() Params {
				params := testParams()
				params.Egress = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: egress limiter is required"),
		},
		{
			name: "missing breaker rejected",
			params: func() Params {
				params := testParams()
				params.Breaker = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: circuit breaker is required"),
		},
		{
			name: "missing coalescer rejected",
			params: func() Params {
				params := testParams()
				params.Coalescer = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: coalescer is required"),
		},
		{
			name: "missing clock rejected",
			params: func() Params {
				params := testParams()
				params.Clock = nil

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: clock is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			assert.Equal(t, tc.expectedResult, provider)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestNewProviderDataAndTiming(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         Params
		expectedResult *Provider
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "missing base url rejected",
			params: func() Params {
				params := testParams()
				params.BaseURL = ""

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): BaseURL: rule "required" on value : validation violation`,
			),
		},
		{
			name: "malformed base url rejected",
			params: func() Params {
				params := testParams()
				params.BaseURL = "not-a-url"

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): BaseURL: rule "url" on value not-a-url: validation violation`,
			),
		},
		{
			name: "missing api key rejected",
			params: func() Params {
				params := testParams()
				params.APIKey = ""

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): APIKey: rule "required" on value : validation violation`,
			),
		},
		{
			name: "missing source rejected",
			params: func() Params {
				params := testParams()
				params.Source = ""

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): Source: rule "required" on value : validation violation`,
			),
		},
		{
			name: "missing live path rejected",
			params: func() Params {
				params := testParams()
				params.LivePath = ""

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): LivePath: rule "required" on value : validation violation`,
			),
		},
		{
			name: "live path without leading slash rejected",
			params: func() Params {
				params := testParams()
				params.LivePath = "live"

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				`fx: params are invalid: fx: invalid params (1 violation(s)): LivePath: rule "startswith" on value live: validation violation`,
			),
		},
		{
			name: "zero fetch timeout rejected",
			params: func() Params {
				params := testParams()
				params.FetchTimeout = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: fetch timeout must be positive"),
		},
		{
			name: "negative fetch timeout rejected",
			params: func() Params {
				params := testParams()
				params.FetchTimeout = -time.Second

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: fetch timeout must be positive"),
		},
		{
			name: "zero max response bytes rejected",
			params: func() Params {
				params := testParams()
				params.MaxResponseBytes = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max response bytes must be positive"),
		},
		{
			name: "negative max response bytes rejected",
			params: func() Params {
				params := testParams()
				params.MaxResponseBytes = -1

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max response bytes must be positive"),
		},
		{
			name: "negative max future skew rejected",
			params: func() Params {
				params := testParams()
				params.MaxFutureSkew = -time.Second

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max future skew must be non-negative"),
		},
		{
			name: "zero max decimal length rejected",
			params: func() Params {
				params := testParams()
				params.MaxDecimalLen = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max decimal length must be positive"),
		},
		{
			name: "negative max decimal length rejected",
			params: func() Params {
				params := testParams()
				params.MaxDecimalLen = -1

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max decimal length must be positive"),
		},
		{
			name: "zero logical ttl rejected",
			params: func() Params {
				params := testParams()
				params.TTL = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: logical ttl must be positive"),
		},
		{
			name: "negative logical ttl rejected",
			params: func() Params {
				params := testParams()
				params.TTL = -time.Second

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: logical ttl must be positive"),
		},
		{
			name: "zero stale retention rejected",
			params: func() Params {
				params := testParams()
				params.StaleRetention = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: stale retention must be positive"),
		},
		{
			name: "stale retention equal to ttl rejected",
			params: func() Params {
				params := testParams()
				params.StaleRetention = params.TTL

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: stale retention must exceed the logical ttl"),
		},
		{
			name: "stale retention below ttl rejected",
			params: func() Params {
				params := testParams()
				params.StaleRetention = params.TTL - time.Second

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: stale retention must exceed the logical ttl"),
		},
		{
			name: "zero max quote age rejected",
			params: func() Params {
				params := testParams()
				params.MaxQuoteAge = 0

				return params
			}(),
			expectedResult: nil,
			expectedError:  entity.NewError(CodeInvalidConfig, "fx: max quote age must be positive"),
		},
		{
			name: "invalid retry policy rejected",
			params: func() Params {
				params := testParams()
				params.RetryPolicy = kernelres.RetryPolicy{}

				return params
			}(),
			expectedResult: nil,
			expectedError: entity.NewError(
				CodeInvalidConfig,
				"fx: retry policy is invalid: resilience: max attempts must be positive",
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			assert.Equal(t, tc.expectedResult, provider)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestNewProviderAcceptsValidParams(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        Params
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid params accepted",
			params:        testParams(),
			expectedError: nil,
		},
		{
			name: "zero max future skew accepted",
			params: func() Params {
				params := testParams()
				params.MaxFutureSkew = 0

				return params
			}(),
			expectedError: nil,
		},
		{
			name: "stale retention one nanosecond above ttl accepted",
			params: func() Params {
				params := testParams()
				params.StaleRetention = params.TTL + time.Nanosecond

				return params
			}(),
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			provider, err := NewProvider(tc.params)
			assert.NotNil(t, provider)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
