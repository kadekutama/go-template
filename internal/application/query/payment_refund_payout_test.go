package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
)

func TestPaymentQueryServiceGetIntent(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		query          port.PaymentQuery
		programmed     port.PaymentIntentResult
		programmedErr  error
		expectedResult port.PaymentIntentResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "intent read delegates to use cases",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "pi-1",
			},
			programmed: port.PaymentIntentResult{
				IntentID: "pi-1",
				Status:   "CONFIRMED",
			},
			programmedErr: nil,
			expectedResult: port.PaymentIntentResult{
				IntentID: "pi-1",
				Status:   "CONFIRMED",
			},
			expectedError: nil,
		},
		{
			name: "missing intent propagates error",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "pi-ghost",
			},
			programmed:     port.PaymentIntentResult{},
			programmedErr:  entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
			expectedResult: port.PaymentIntentResult{},
			expectedError:  entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payments := new(mockapplication.MockPaymentQueryUseCases)
			payments.On("GetIntent", mock.Anything, tc.query).Return(tc.programmed, tc.programmedErr).Once()
			svc := query.NewPaymentQueryService(query.PaymentQueryServiceParams{Payments: payments})
			actualResult, err := svc.GetIntent(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			payments.AssertExpectations(t)
		})
	}
}

func TestRefundQueryServiceGetRefund(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		query          port.PaymentQuery
		programmed     port.RefundResult
		programmedErr  error
		expectedResult port.RefundResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "refund read delegates to use cases",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "re-1",
			},
			programmed: port.RefundResult{
				RefundID: "re-1",
				Status:   "SUCCEEDED",
			},
			programmedErr: nil,
			expectedResult: port.RefundResult{
				RefundID: "re-1",
				Status:   "SUCCEEDED",
			},
			expectedError: nil,
		},
		{
			name: "refund not found propagates error",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "re-ghost",
			},
			programmed:     port.RefundResult{},
			programmedErr:  entity.NewError("REFUND_NOT_FOUND", "refund is unknown"),
			expectedResult: port.RefundResult{},
			expectedError:  entity.NewError("REFUND_NOT_FOUND", "refund is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payments := new(mockapplication.MockPaymentQueryUseCases)
			payments.On("GetRefund", mock.Anything, tc.query).Return(tc.programmed, tc.programmedErr).Once()
			svc := query.NewRefundQueryService(query.RefundQueryServiceParams{Payments: payments})
			actualResult, err := svc.GetRefund(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			payments.AssertExpectations(t)
		})
	}
}

func TestPayoutQueryServiceGetPayout(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		query          port.PaymentQuery
		programmed     port.PayoutResult
		programmedErr  error
		expectedResult port.PayoutResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "payout read delegates to use cases",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "po-1",
			},
			programmed: port.PayoutResult{
				PayoutID: "po-1",
				Status:   "COMPLETED",
			},
			programmedErr: nil,
			expectedResult: port.PayoutResult{
				PayoutID: "po-1",
				Status:   "COMPLETED",
			},
			expectedError: nil,
		},
		{
			name: "payout not found propagates error",
			query: port.PaymentQuery{
				TenantID: "t-1",
				ID:       "po-ghost",
			},
			programmed:     port.PayoutResult{},
			programmedErr:  entity.NewError("PAYOUT_NOT_FOUND", "payout is unknown"),
			expectedResult: port.PayoutResult{},
			expectedError:  entity.NewError("PAYOUT_NOT_FOUND", "payout is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payments := new(mockapplication.MockPaymentQueryUseCases)
			payments.On("GetPayout", mock.Anything, tc.query).Return(tc.programmed, tc.programmedErr).Once()
			svc := query.NewPayoutQueryService(query.PayoutQueryServiceParams{Payments: payments})
			actualResult, err := svc.GetPayout(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			payments.AssertExpectations(t)
		})
	}
}
