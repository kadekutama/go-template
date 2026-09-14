package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseTopupStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.TopupStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending",
			s:              "PENDING",
			expectedResult: valueobject.TopupPending,
			expectedError:  nil,
		},
		{
			name:           "succeeded",
			s:              "SUCCEEDED",
			expectedResult: valueobject.TopupSucceeded,
			expectedError:  nil,
		},
		{
			name:           "failed",
			s:              "FAILED",
			expectedResult: valueobject.TopupFailed,
			expectedError:  nil,
		},
		{
			name:           "canceled",
			s:              "CANCELED",
			expectedResult: valueobject.TopupCanceled,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "SETTLED",
			expectedResult: valueobject.TopupStatus(""),
			expectedError:  errors.New(`topup: invalid status "SETTLED"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.TopupStatus(""),
			expectedError:  errors.New(`topup: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParseTopupStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionTopup(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.TopupStatus
		to             valueobject.TopupStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "pending to succeeded",
			from:           valueobject.TopupPending,
			to:             valueobject.TopupSucceeded,
			expectedResult: true,
		},
		{
			name:           "pending to failed",
			from:           valueobject.TopupPending,
			to:             valueobject.TopupFailed,
			expectedResult: true,
		},
		{
			name:           "pending to canceled",
			from:           valueobject.TopupPending,
			to:             valueobject.TopupCanceled,
			expectedResult: true,
		},
		{
			name:           "succeeded to failed (illegal)",
			from:           valueobject.TopupSucceeded,
			to:             valueobject.TopupFailed,
			expectedResult: false,
		},
		{
			name:           "failed to pending (illegal)",
			from:           valueobject.TopupFailed,
			to:             valueobject.TopupPending,
			expectedResult: false,
		},
		{
			name:           "canceled to succeeded (illegal)",
			from:           valueobject.TopupCanceled,
			to:             valueobject.TopupSucceeded,
			expectedResult: false,
		},
		{
			name:           "unknown from status (illegal)",
			from:           valueobject.TopupStatus("UNKNOWN"),
			to:             valueobject.TopupSucceeded,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := valueobject.CanTransitionTopup(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
