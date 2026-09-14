package resilience

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultClassifier(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		err            error
		expectedResult Class
	}

	testCases := []testCase{
		{
			name:           "retryable error classified",
			err:            &RetryableError{Err: errors.New("x")},
			expectedResult: Retryable,
		},
		{
			name:           "permanent error classified",
			err:            &PermanentError{Err: errors.New("x")},
			expectedResult: Permanent,
		},
		{
			name:           "standard error classified as unknown",
			err:            errors.New("x"),
			expectedResult: Unknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := DefaultClassifier(tc.err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestShouldRetry(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		class          Class
		idempotent     bool
		key            string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "permanent never retries even with key",
			class:          Permanent,
			idempotent:     true,
			key:            "k",
			expectedResult: false,
		},
		{
			name:           "retryable with key retries",
			class:          Retryable,
			idempotent:     false,
			key:            "k",
			expectedResult: true,
		},
		{
			name:           "retryable idempotent retries",
			class:          Retryable,
			idempotent:     true,
			key:            "",
			expectedResult: true,
		},
		{
			name:           "retryable without key or flag never retries",
			class:          Retryable,
			idempotent:     false,
			key:            "",
			expectedResult: false,
		},
		{
			name:           "unknown with key retries",
			class:          Unknown,
			idempotent:     false,
			key:            "k",
			expectedResult: true,
		},
		{
			name:           "unknown without key never retries",
			class:          Unknown,
			idempotent:     false,
			key:            "",
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := ShouldRetry(tc.class, tc.idempotent, tc.key)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestBackoffFor(t *testing.T) {
	t.Parallel()

	policy := RetryPolicy{
		MaxAttempts:    5,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     250 * time.Millisecond,
		Multiplier:     10,
	}.Normalize()

	type testCase struct {
		name           string
		attempt        int
		expectedResult time.Duration
	}

	testCases := []testCase{
		{
			name:           "zero attempt backoff",
			attempt:        0,
			expectedResult: 100 * time.Millisecond,
		},
		{
			name:           "negative attempt backoff floors at initial",
			attempt:        -1,
			expectedResult: 100 * time.Millisecond,
		},
		{
			name:           "first attempt backoff",
			attempt:        1,
			expectedResult: 100 * time.Millisecond,
		},
		{
			name:           "capped at max backoff",
			attempt:        4,
			expectedResult: 250 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := policy.BackoffFor(tc.attempt)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestNormalizePolicy(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		policy         RetryPolicy
		expectedResult RetryPolicy
	}

	testCases := []testCase{
		{
			name:   "zero policy defaulted",
			policy: RetryPolicy{},
			expectedResult: RetryPolicy{
				MaxAttempts:    DefaultMaxAttempts,
				InitialBackoff: DefaultInitialBackoff,
				MaxBackoff:     DefaultMaxBackoff,
				Multiplier:     DefaultMultiplier,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.policy.Normalize()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestErrorWrappers(t *testing.T) {
	t.Parallel()

	inner := errors.New("underlying issue")

	type testCase struct {
		name           string
		err            error
		expectedError  string
		expectedUnwrap error
	}

	testCases := []testCase{
		{
			name:           "retryable error unwraps",
			err:            &RetryableError{Err: inner},
			expectedError:  "underlying issue",
			expectedUnwrap: inner,
		},
		{
			name:           "permanent error unwraps",
			err:            &PermanentError{Err: inner},
			expectedError:  "underlying issue",
			expectedUnwrap: inner,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedError, tc.err.Error())
			assert.ErrorIs(t, tc.err, tc.expectedUnwrap)
			if unwrap, ok := tc.err.(interface{ Unwrap() error }); ok {
				assert.Equal(t, tc.expectedUnwrap, unwrap.Unwrap())
			}
		})
	}
}
