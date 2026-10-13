package egress_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

type scriptStep struct {
	decision appport.RateLimitDecision
	err      error
}

// scriptedLimiter serves a canned Allow sequence through the generated
// mockery mock, recording every call for key and count assertions.
type scriptedLimiter struct {
	mock   *mockapplication.MockRateLimiter
	script []scriptStep
}

func newScriptedLimiter(script []scriptStep) *scriptedLimiter {
	limiter := &scriptedLimiter{mock: &mockapplication.MockRateLimiter{}, script: script}
	limiter.mock.On(
		"Allow",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(func(_ context.Context, _ string, _ int64, _ time.Duration) (appport.RateLimitDecision, error) {
		if len(limiter.script) == 0 {
			return appport.RateLimitDecision{Allowed: true}, nil
		}

		step := limiter.script[0]
		limiter.script = limiter.script[1:]

		return step.decision, step.err
	})

	return limiter
}

func (f *scriptedLimiter) calls() []mock.Call {
	return f.mock.Calls
}

func (f *scriptedLimiter) keys() []string {
	keys := make([]string, 0, len(f.mock.Calls))
	for _, call := range f.mock.Calls {
		key, _ := call.Arguments.Get(1).(string)
		keys = append(keys, key)
	}

	return keys
}

// mockClock scripts time through testify expectations. Callers pass either
// fixed instants or a func reading shared test state, so sleeps can advance
// the observed clock deterministically.
type mockClock struct {
	mock.Mock
}

func (m *mockClock) Now() time.Time {
	args := m.Called()
	if fn, ok := args.Get(0).(func() time.Time); ok {
		return fn()
	}

	return args.Get(0).(time.Time)
}

// mockJitter scripts deterministic jitter through testify expectations.
type mockJitter struct {
	mock.Mock
}

func (m *mockJitter) Jitter(upper time.Duration) time.Duration {
	args := m.Called(upper)
	delay, _ := args.Get(0).(time.Duration)

	return delay
}

// mockSleeper scripts sleeps through testify expectations.
type mockSleeper struct {
	mock.Mock
}

func (m *mockSleeper) Sleep(ctx context.Context, d time.Duration) error {
	args := m.Called(ctx, d)

	return args.Error(0)
}

func testPolicy() egress.Policy {
	return egress.Policy{
		Provider:  "prov",
		Operation: "op",
		Budget:    10,
		Window:    time.Minute,
		MaxWait:   150 * time.Millisecond,
	}
}

func testLimiter(t *testing.T, policy egress.Policy, script []scriptStep) (*egress.Limiter, *scriptedLimiter, *mockSleeper) {
	t.Helper()

	limiter := newScriptedLimiter(script)
	now := time.Now()
	clock := &mockClock{}
	clock.On("Now").Return(func() time.Time { return now })

	jitter := &mockJitter{}
	jitter.On("Jitter", mock.Anything).Return(time.Duration(0))

	sleeper := &mockSleeper{}
	sleeper.On("Sleep", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		now = now.Add(args.Get(1).(time.Duration))
	}).Return(nil)
	waiter, err := egress.NewEgressLimiter(egress.Params{
		Limiter:  limiter.mock,
		Policies: []egress.Policy{policy},
		Clock:    clock,
		Jitter:   jitter,
		Sleeper:  sleeper,
	})
	require.NoError(t, err)

	return waiter, limiter, sleeper
}

func errorCode(t *testing.T, err error) string {
	t.Helper()

	var egressErr *entity.Error
	require.ErrorAs(t, err, &egressErr)

	return egressErr.Code
}

func TestNewEgressLimiterValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		params            egress.Params
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "missing limiter rejected",
			params: egress.Params{
				Policies: []egress.Policy{testPolicy()},
			},
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name:              "empty policies rejected",
			params:            egress.Params{Limiter: newScriptedLimiter(nil).mock},
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "zero budget rejected",
			params: func() egress.Params {
				policy := testPolicy()
				policy.Budget = 0

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "zero window rejected",
			params: func() egress.Params {
				policy := testPolicy()
				policy.Window = 0

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "zero max wait rejected",
			params: func() egress.Params {
				policy := testPolicy()
				policy.MaxWait = 0

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "empty provider rejected",
			params: func() egress.Params {
				policy := testPolicy()
				policy.Provider = ""

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "colon operation rejected",
			params: func() egress.Params {
				policy := testPolicy()
				policy.Operation = "a:b"

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "duplicate policy rejected",
			params: func() egress.Params {
				policy := testPolicy()

				return egress.Params{Limiter: newScriptedLimiter(nil).mock, Policies: []egress.Policy{policy, policy}}
			}(),
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			waiter, err := egress.NewEgressLimiter(tc.params)
			assert.Nil(t, waiter)
			require.Error(t, err)
			assert.Equal(t, tc.expectedErrorCode, errorCode(t, err))
		})
	}
}

func TestEgressLimiterWait(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		provider          string
		operation         string
		policy            egress.Policy
		script            []scriptStep
		cancelBefore      bool
		cancelDuringSleep bool
		expectedErrorCode string
		expectedAllows    int
		expectCanceled    bool
	}

	testCases := []testCase{
		{
			name:              "immediate admission",
			provider:          "prov",
			operation:         "op",
			policy:            testPolicy(),
			script:            []scriptStep{{decision: appport.RateLimitDecision{Allowed: true, Remaining: 9}}},
			expectedErrorCode: "",
			expectedAllows:    1,
		},
		{
			name:      "denied then admitted",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: 100 * time.Millisecond}},
				{decision: appport.RateLimitDecision{Allowed: true, Remaining: 9}},
			},
			expectedErrorCode: "",
			expectedAllows:    2,
		},
		{
			name:      "wait exceeded",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: 100 * time.Millisecond}},
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: 100 * time.Millisecond}},
			},
			expectedErrorCode: egress.CodeWaitExceeded,
			expectedAllows:    2,
		},
		{
			name:      "retry longer than max wait never sleeps",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: time.Minute}},
			},
			expectedErrorCode: egress.CodeWaitExceeded,
			expectedAllows:    1,
		},
		{
			name:      "zero retry delay is malformed",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: 0}},
			},
			expectedErrorCode: egress.CodeMalformedDecision,
			expectedAllows:    1,
		},
		{
			name:      "negative retry delay is malformed",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: -time.Second}},
			},
			expectedErrorCode: egress.CodeMalformedDecision,
			expectedAllows:    1,
		},
		{
			name:      "store error is terminal",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{}, err: egress.ErrValkeyDown},
			},
			expectedErrorCode: egress.CodeStoreUnavailable,
			expectedAllows:    1,
		},
		{
			name:              "missing policy fails closed",
			provider:          "unknown",
			operation:         "op",
			policy:            testPolicy(),
			expectedErrorCode: egress.CodeInvalidPolicy,
			expectedAllows:    0,
		},
		{
			name:              "empty provider fails before port",
			provider:          "",
			operation:         "op",
			policy:            testPolicy(),
			expectedErrorCode: egress.CodeInvalidPolicy,
			expectedAllows:    0,
		},
		{
			name:              "pre-canceled context performs no port call",
			provider:          "prov",
			operation:         "op",
			policy:            testPolicy(),
			script:            []scriptStep{{decision: appport.RateLimitDecision{Allowed: true, Remaining: 9}}},
			cancelBefore:      true,
			expectedErrorCode: egress.CodeContextCanceled,
			expectedAllows:    0,
			expectCanceled:    true,
		},
		{
			name:      "cancel during sleep stops before another allow",
			provider:  "prov",
			operation: "op",
			policy:    testPolicy(),
			script: []scriptStep{
				{decision: appport.RateLimitDecision{Allowed: false, RetryAfter: 100 * time.Millisecond}},
				{decision: appport.RateLimitDecision{Allowed: true, Remaining: 9}},
			},
			cancelDuringSleep: true,
			expectedErrorCode: egress.CodeContextCanceled,
			expectedAllows:    1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			waiter, limiter, sleeper := testLimiter(t, tc.policy, tc.script)
			ctx := context.Background()

			if tc.cancelBefore {
				var cancel context.CancelFunc

				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}

			if tc.cancelDuringSleep {
				sleeper.ExpectedCalls = nil
				sleeper.On("Sleep", mock.Anything, mock.Anything).Return(context.Canceled)
			}

			err := waiter.Wait(ctx, tc.provider, tc.operation)
			if tc.expectedErrorCode == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.expectedErrorCode, errorCode(t, err))
			}

			if tc.expectCanceled {
				assert.ErrorIs(t, err, context.Canceled)
			}

			assert.Len(t, limiter.calls(), tc.expectedAllows)

			for _, key := range limiter.keys() {
				assert.Equal(t, "ratelimit:prov:op", key)
			}
		})
	}
}

func TestParsePolicies(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		document          string
		expectedCount     int
		expectedErrorCode string
	}

	testCases := []testCase{
		{
			name: "valid document",
			document: "policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 100\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n",
			expectedCount:     1,
			expectedErrorCode: "",
		},
		{
			name:              "empty document rejected",
			document:          "policies: []\n",
			expectedCount:     0,
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name:              "malformed yaml rejected",
			document:          "policies: [unclosed\n",
			expectedCount:     0,
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "zero budget rejected",
			document: "policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 0\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n",
			expectedCount:     0,
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "bad duration rejected",
			document: "policies:\n" +
				"  - provider: unleash\n" +
				"    operation: flags\n" +
				"    budget: 10\n" +
				"    window: soon\n" +
				"    max_wait: 5s\n",
			expectedCount:     0,
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
		{
			name: "missing provider rejected",
			document: "policies:\n" +
				"  - operation: flags\n" +
				"    budget: 10\n" +
				"    window: 1m\n" +
				"    max_wait: 5s\n",
			expectedCount:     0,
			expectedErrorCode: egress.CodeInvalidPolicy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			policies, err := egress.ParsePolicies([]byte(tc.document))
			if tc.expectedErrorCode == "" {
				require.NoError(t, err)
				assert.Len(t, policies, tc.expectedCount)

				return
			}

			require.Error(t, err)
			assert.Equal(t, tc.expectedErrorCode, errorCode(t, err))
		})
	}
}
