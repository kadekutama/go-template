package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParsePaymentStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.PaymentStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "requires method",
			s:              "REQUIRES_METHOD",
			expectedResult: valueobject.PaymentRequiresMethod,
			expectedError:  nil,
		},
		{
			name:           "requires action",
			s:              "REQUIRES_ACTION",
			expectedResult: valueobject.PaymentRequiresAction,
			expectedError:  nil,
		},
		{
			name:           "authorized",
			s:              "AUTHORIZED",
			expectedResult: valueobject.PaymentAuthorized,
			expectedError:  nil,
		},
		{
			name:           "captured",
			s:              "CAPTURED",
			expectedResult: valueobject.PaymentCaptured,
			expectedError:  nil,
		},
		{
			name:           "pending settlement",
			s:              "PENDING_SETTLEMENT",
			expectedResult: valueobject.PaymentPendingSettle,
			expectedError:  nil,
		},
		{
			name:           "settled",
			s:              "SETTLED",
			expectedResult: valueobject.PaymentSettled,
			expectedError:  nil,
		},
		{
			name:           "failed",
			s:              "FAILED",
			expectedResult: valueobject.PaymentFailed,
			expectedError:  nil,
		},
		{
			name:           "returned",
			s:              "RETURNED",
			expectedResult: valueobject.PaymentReturned,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "UNKNOWN",
			expectedResult: valueobject.PaymentStatus(""),
			expectedError:  errors.New(`payment: invalid status "UNKNOWN"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.PaymentStatus(""),
			expectedError:  errors.New(`payment: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParsePaymentStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionPayment(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.PaymentStatus
		to             valueobject.PaymentStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "requires method to requires action",
			from:           valueobject.PaymentRequiresMethod,
			to:             valueobject.PaymentRequiresAction,
			expectedResult: true,
		},
		{
			name:           "requires method to authorized",
			from:           valueobject.PaymentRequiresMethod,
			to:             valueobject.PaymentAuthorized,
			expectedResult: true,
		},
		{
			name:           "requires method to failed",
			from:           valueobject.PaymentRequiresMethod,
			to:             valueobject.PaymentFailed,
			expectedResult: true,
		},
		{
			name:           "requires action to authorized",
			from:           valueobject.PaymentRequiresAction,
			to:             valueobject.PaymentAuthorized,
			expectedResult: true,
		},
		{
			name:           "requires action to failed",
			from:           valueobject.PaymentRequiresAction,
			to:             valueobject.PaymentFailed,
			expectedResult: true,
		},
		{
			name:           "authorized to captured",
			from:           valueobject.PaymentAuthorized,
			to:             valueobject.PaymentCaptured,
			expectedResult: true,
		},
		{
			name:           "authorized to failed",
			from:           valueobject.PaymentAuthorized,
			to:             valueobject.PaymentFailed,
			expectedResult: true,
		},
		{
			name:           "captured to pending settlement",
			from:           valueobject.PaymentCaptured,
			to:             valueobject.PaymentPendingSettle,
			expectedResult: true,
		},
		{
			name:           "captured to failed",
			from:           valueobject.PaymentCaptured,
			to:             valueobject.PaymentFailed,
			expectedResult: true,
		},
		{
			name:           "pending settlement to settled",
			from:           valueobject.PaymentPendingSettle,
			to:             valueobject.PaymentSettled,
			expectedResult: true,
		},
		{
			name:           "pending settlement to failed",
			from:           valueobject.PaymentPendingSettle,
			to:             valueobject.PaymentFailed,
			expectedResult: true,
		},
		{
			name:           "pending settlement to returned",
			from:           valueobject.PaymentPendingSettle,
			to:             valueobject.PaymentReturned,
			expectedResult: true,
		},
		{
			name:           "settled to failed (illegal)",
			from:           valueobject.PaymentSettled,
			to:             valueobject.PaymentFailed,
			expectedResult: false,
		},
		{
			name:           "failed to authorized (illegal)",
			from:           valueobject.PaymentFailed,
			to:             valueobject.PaymentAuthorized,
			expectedResult: false,
		},
		{
			name:           "returned to settled (illegal)",
			from:           valueobject.PaymentReturned,
			to:             valueobject.PaymentSettled,
			expectedResult: false,
		},
		{
			name:           "unknown from status (illegal)",
			from:           valueobject.PaymentStatus("UNKNOWN"),
			to:             valueobject.PaymentSettled,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := valueobject.CanTransitionPayment(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
