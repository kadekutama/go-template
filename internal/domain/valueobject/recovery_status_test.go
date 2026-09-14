package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseRecoveryStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.RecoveryStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending",
			s:              "PENDING",
			expectedResult: valueobject.RecoveryPending,
			expectedError:  nil,
		},
		{
			name:           "collected",
			s:              "COLLECTED",
			expectedResult: valueobject.RecoveryCollected,
			expectedError:  nil,
		},
		{
			name:           "failed",
			s:              "FAILED",
			expectedResult: valueobject.RecoveryFailed,
			expectedError:  nil,
		},
		{
			name:           "canceled",
			s:              "CANCELED",
			expectedResult: valueobject.RecoveryCanceled,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "SETTLED",
			expectedResult: valueobject.RecoveryStatus(""),
			expectedError:  errors.New(`recovery: invalid status "SETTLED"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.RecoveryStatus(""),
			expectedError:  errors.New(`recovery: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := valueobject.ParseRecoveryStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionRecovery(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.RecoveryStatus
		to             valueobject.RecoveryStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "pending to collected",
			from:           valueobject.RecoveryPending,
			to:             valueobject.RecoveryCollected,
			expectedResult: true,
		},
		{
			name:           "pending to failed",
			from:           valueobject.RecoveryPending,
			to:             valueobject.RecoveryFailed,
			expectedResult: true,
		},
		{
			name:           "pending to canceled",
			from:           valueobject.RecoveryPending,
			to:             valueobject.RecoveryCanceled,
			expectedResult: true,
		},
		{
			name:           "collected to canceled (illegal)",
			from:           valueobject.RecoveryCollected,
			to:             valueobject.RecoveryCanceled,
			expectedResult: false,
		},
		{
			name:           "failed to pending (illegal)",
			from:           valueobject.RecoveryFailed,
			to:             valueobject.RecoveryPending,
			expectedResult: false,
		},
		{
			name:           "canceled to collected (illegal)",
			from:           valueobject.RecoveryCanceled,
			to:             valueobject.RecoveryCollected,
			expectedResult: false,
		},
		{
			name:           "unknown from (illegal)",
			from:           valueobject.RecoveryStatus("UNKNOWN"),
			to:             valueobject.RecoveryCollected,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := valueobject.CanTransitionRecovery(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
