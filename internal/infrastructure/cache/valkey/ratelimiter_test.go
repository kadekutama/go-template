package valkey_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
)

func TestNewRateLimiter(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         valkey.LimiterParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "client required",
			params: func() valkey.LimiterParams {
				client, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
				if err != nil {
					panic(err)
				}
				return valkey.LimiterParams{Client: client}
			}(),
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil client rejected",
			params: valkey.LimiterParams{
				Client: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("ratelimit: valkey client is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			limiter, err := valkey.NewRateLimiter(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, limiter)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, limiter)
			assert.Equal(t, tc.expectedResult, limiter != nil)
		})
	}
}

// fakeRunner returns canned Lua results for Allow-decision tests.
type fakeRunner struct {
	result []any
	err    error
}

func (f *fakeRunner) Eval(_ context.Context, _ string, _ []string, _ ...any) (any, error) {
	if f.err != nil {
		return nil, f.err
	}

	return f.result, nil
}

func TestRateLimiterAllowDecisions(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		key            string
		budget         int64
		window         time.Duration
		runner         *fakeRunner
		expectedResult appport.RateLimitDecision
		expectedError  error
		expectAllowed  int
		expectDenied   int
	}

	testCases := []testCase{
		{
			name:   "allowed decision with hooks",
			ctx:    context.Background(),
			key:    "ratelimit:t1:u1",
			budget: 10,
			window: time.Minute,
			runner: &fakeRunner{
				result: []any{int64(1), int64(9), int64(60000)},
			},
			expectedResult: appport.RateLimitDecision{
				Allowed:    true,
				Remaining:  9,
				RetryAfter: time.Minute,
			},
			expectedError: nil,
			expectAllowed: 1,
			expectDenied:  0,
		},
		{
			name:   "denied decision with retry hint",
			ctx:    context.Background(),
			key:    "ratelimit:t1:u2",
			budget: 10,
			window: time.Minute,
			runner: &fakeRunner{
				result: []any{int64(0), int64(0), int64(15000)},
			},
			expectedResult: appport.RateLimitDecision{
				Allowed:    false,
				Remaining:  0,
				RetryAfter: 15 * time.Second,
			},
			expectedError: nil,
			expectAllowed: 0,
			expectDenied:  1,
		},
		{
			name:   "store error surfaces",
			ctx:    context.Background(),
			key:    "ratelimit:t1:u3",
			budget: 10,
			window: time.Minute,
			runner: &fakeRunner{
				err: errors.New("valkey down"),
			},
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("valkey down"),
			expectAllowed:  0,
			expectDenied:   0,
		},
		{
			name:   "malformed script result rejected",
			ctx:    context.Background(),
			key:    "ratelimit:t1:u4",
			budget: 10,
			window: time.Minute,
			runner: &fakeRunner{
				result: []any{int64(1)},
			},
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: unexpected script result []interface {}"),
			expectAllowed:  0,
			expectDenied:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed, denied := 0, 0
			limiter, err := valkey.NewRateLimiter(valkey.LimiterParams{
				Client:  tc.runner,
				OnAllow: func(_ context.Context, _ string) { allowed++ },
				OnDeny:  func(_ context.Context, _ string) { denied++ },
			})
			require.NoError(t, err)

			actualResult, err := limiter.Allow(tc.ctx, tc.key, tc.budget, tc.window)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectAllowed, allowed)
			assert.Equal(t, tc.expectDenied, denied)
		})
	}
}

func TestRateLimiterAllowValidation(t *testing.T) {
	t.Parallel()

	validClient, err := valkey.NewValkeyClient(valkey.ValkeyParams{Addr: "127.0.0.1:6379"})
	require.NoError(t, err)
	validLimiter, err := valkey.NewRateLimiter(valkey.LimiterParams{Client: validClient})
	require.NoError(t, err)

	type testCase struct {
		name           string
		limiter        *valkey.RateLimiter
		ctx            context.Context
		key            string
		budget         int64
		window         time.Duration
		expectedResult appport.RateLimitDecision
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "nil receiver rejected",
			limiter:        nil,
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         5,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: not initialized"),
		},
		{
			name:           "uninitialized internal client rejected",
			limiter:        &valkey.RateLimiter{},
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         5,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: not initialized"),
		},
		{
			name:           "empty key rejected",
			limiter:        validLimiter,
			ctx:            context.Background(),
			key:            "",
			budget:         5,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: key is required"),
		},
		{
			name:           "zero budget rejected",
			limiter:        validLimiter,
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         0,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: budget must be positive"),
		},
		{
			name:           "negative budget rejected",
			limiter:        validLimiter,
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         -1,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: budget must be positive"),
		},
		{
			name:           "zero window rejected",
			limiter:        validLimiter,
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         5,
			window:         0,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: window must be positive"),
		},
		{
			name:           "negative window rejected",
			limiter:        validLimiter,
			ctx:            context.Background(),
			key:            "ratelimit:t1:u1",
			budget:         5,
			window:         -time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  errors.New("ratelimit: window must be positive"),
		},
		{
			name:    "canceled context aborts",
			limiter: validLimiter,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			key:            "ratelimit:t1:u1",
			budget:         5,
			window:         time.Minute,
			expectedResult: appport.RateLimitDecision{},
			expectedError:  context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := tc.limiter.Allow(tc.ctx, tc.key, tc.budget, tc.window)
			if tc.expectedError != nil {
				require.Error(t, err)
				if errors.Is(tc.expectedError, context.Canceled) {
					assert.ErrorIs(t, err, context.Canceled)
				} else {
					assert.EqualError(t, err, tc.expectedError.Error())
				}
				assert.Equal(t, tc.expectedResult, actualResult)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
