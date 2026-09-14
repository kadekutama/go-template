package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParsePayoutStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.PayoutStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending",
			s:              "PENDING",
			expectedResult: valueobject.PayoutPending,
			expectedError:  nil,
		},
		{
			name:           "in transit",
			s:              "IN_TRANSIT",
			expectedResult: valueobject.PayoutInTransit,
			expectedError:  nil,
		},
		{
			name:           "paid",
			s:              "PAID",
			expectedResult: valueobject.PayoutPaid,
			expectedError:  nil,
		},
		{
			name:           "failed",
			s:              "FAILED",
			expectedResult: valueobject.PayoutFailed,
			expectedError:  nil,
		},
		{
			name:           "canceled",
			s:              "CANCELED",
			expectedResult: valueobject.PayoutCanceled,
			expectedError:  nil,
		},
		{
			name:           "unknown status",
			s:              "REJECTED",
			expectedResult: valueobject.PayoutStatus(""),
			expectedError:  errors.New(`payout: invalid status "REJECTED"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.PayoutStatus(""),
			expectedError:  errors.New(`payout: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParsePayoutStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestPayoutCanTransition(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.PayoutStatus
		to             valueobject.PayoutStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "pending to in transit",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutInTransit,
			expectedResult: true,
		},
		{
			name:           "pending to canceled",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutCanceled,
			expectedResult: true,
		},
		{
			name:           "in transit to paid",
			from:           valueobject.PayoutInTransit,
			to:             valueobject.PayoutPaid,
			expectedResult: true,
		},
		{
			name:           "in transit to failed",
			from:           valueobject.PayoutInTransit,
			to:             valueobject.PayoutFailed,
			expectedResult: true,
		},
		{
			name:           "pending to paid (illegal)",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutPaid,
			expectedResult: false,
		},
		{
			name:           "pending to failed (illegal)",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutFailed,
			expectedResult: false,
		},
		{
			name:           "in transit to canceled (illegal)",
			from:           valueobject.PayoutInTransit,
			to:             valueobject.PayoutCanceled,
			expectedResult: false,
		},
		{
			name:           "paid to failed (illegal)",
			from:           valueobject.PayoutPaid,
			to:             valueobject.PayoutFailed,
			expectedResult: false,
		},
		{
			name:           "failed to pending (illegal)",
			from:           valueobject.PayoutFailed,
			to:             valueobject.PayoutPending,
			expectedResult: false,
		},
		{
			name:           "canceled to in transit (illegal)",
			from:           valueobject.PayoutCanceled,
			to:             valueobject.PayoutInTransit,
			expectedResult: false,
		},
		{
			name:           "unknown from (illegal)",
			from:           valueobject.PayoutStatus("UNKNOWN"),
			to:             valueobject.PayoutPaid,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := valueobject.CanTransition(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestCanCancelPayout(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              valueobject.PayoutStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "pending is cancelable",
			s:              valueobject.PayoutPending,
			expectedResult: true,
		},
		{
			name:           "in transit is not cancelable",
			s:              valueobject.PayoutInTransit,
			expectedResult: false,
		},
		{
			name:           "paid is not cancelable",
			s:              valueobject.PayoutPaid,
			expectedResult: false,
		},
		{
			name:           "failed is not cancelable",
			s:              valueobject.PayoutFailed,
			expectedResult: false,
		},
		{
			name:           "canceled is not cancelable",
			s:              valueobject.PayoutCanceled,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := valueobject.CanCancel(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
