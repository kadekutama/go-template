package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateSubmission(t *testing.T) {
	t.Parallel()

	baseSub := service.PayoutSubmission{
		MerchantAccount: "m-avail",
		PayoutsPayable:  "p-pay",
		AmountMinor:     10000,
		AssetCode:       "USD",
		Method:          valueobject.PayoutACH,
		Reference:       service.PayoutReference{ProviderReference: "pr-1", TraceID: "tr-1", IdempotencyKey: "k-1"},
	}

	type testCase struct {
		name           string
		sub            service.PayoutSubmission
		expectedResult service.PayoutSubmission
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ok",
			sub:            baseSub,
			expectedResult: baseSub,
			expectedError:  nil,
		},
		{
			name: "bad method",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.Method = "NOPE"
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "PAYOUT_METHOD_UNKNOWN", Message: "payout method is unknown"},
		},
		{
			name: "missing account",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.MerchantAccount = ""
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "PAYOUT_ACCOUNT_REQUIRED", Message: "payout requires merchant and payouts-payable accounts"},
		},
		{
			name: "identical accounts",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.MerchantAccount = s.PayoutsPayable
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "PAYOUT_ACCOUNT_INVALID", Message: "payout staged accounts must be distinct"},
		},
		{
			name: "missing asset",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.AssetCode = ""
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "PAYOUT_ASSET_REQUIRED", Message: "payout requires an asset code"},
		},
		{
			name: "missing key",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.Reference.IdempotencyKey = ""
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "IDEMPOTENCY_KEY_REQUIRED", Message: "payout requires an idempotency key"},
		},
		{
			name: "zero amount",
			sub: func() service.PayoutSubmission {
				s := baseSub
				s.AmountMinor = 0
				return s
			}(),
			expectedResult: service.PayoutSubmission{},
			expectedError:  &entity.Error{Code: "INVALID_PAYOUT_AMOUNT", Message: "payout amount must be positive"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateSubmission(tc.sub)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateSettlement(t *testing.T) {
	t.Parallel()

	sub := service.PayoutSubmission{
		MerchantAccount: "m-avail",
		PayoutsPayable:  "p-pay",
		AmountMinor:     10000,
		AssetCode:       "USD",
		Method:          valueobject.PayoutACH,
		Reference:       service.PayoutReference{ProviderReference: "pr-1", TraceID: "tr-1", IdempotencyKey: "k-1"},
	}

	settledAt := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	baseStl := service.PayoutSettlement{
		PayoutsPayable: "p-pay",
		BankCash:       "bank",
		AmountMinor:    10000,
		AssetCode:      "USD",
		Method:         valueobject.PayoutACH,
		SettledAt:      settledAt,
	}

	type testCase struct {
		name           string
		sub            service.PayoutSubmission
		stl            service.PayoutSettlement
		expectedResult service.PayoutSettlement
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ok",
			sub:            sub,
			stl:            baseStl,
			expectedResult: baseStl,
			expectedError:  nil,
		},
		{
			name: "amount mismatch",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.AmountMinor = 9999
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "PAYOUT_SETTLEMENT_MISMATCH", Message: "settlement must match submission amount and asset"},
		},
		{
			name: "rail mismatch",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.Method = valueobject.PayoutWire
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "PAYOUT_SETTLEMENT_MISMATCH", Message: "settlement must use the submission rail"},
		},
		{
			name: "wrong payable",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.PayoutsPayable = "other"
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "PAYOUT_SETTLEMENT_MISMATCH", Message: "settlement must clear the submission payable"},
		},
		{
			name: "missing cash",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.BankCash = ""
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "PAYOUT_ACCOUNT_REQUIRED", Message: "settlement requires a bank cash account"},
		},
		{
			name: "identical accounts",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.BankCash = "p-pay"
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "PAYOUT_ACCOUNT_INVALID", Message: "settlement accounts must be distinct"},
		},
		{
			name: "zero time",
			sub:  sub,
			stl: func() service.PayoutSettlement {
				s := baseStl
				s.SettledAt = time.Time{}
				return s
			}(),
			expectedResult: service.PayoutSettlement{},
			expectedError:  &entity.Error{Code: "SETTLED_AT_REQUIRED", Message: "settlement requires a settlement timestamp"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateSettlement(tc.sub, tc.stl)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCancelPayout(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		status         valueobject.PayoutStatus
		expectedResult valueobject.PayoutStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending cancels",
			status:         valueobject.PayoutPending,
			expectedResult: valueobject.PayoutCanceled,
			expectedError:  nil,
		},
		{
			name:           "in-transit rejected",
			status:         valueobject.PayoutInTransit,
			expectedResult: valueobject.PayoutInTransit,
			expectedError:  &entity.Error{Code: "PAYOUT_CANCEL_REJECTED", Message: "payout can be canceled only while PENDING"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.CancelPayout(tc.status)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransitionPayout(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.PayoutStatus
		to             valueobject.PayoutStatus
		expectedResult valueobject.PayoutStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "pending to in-transit",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutInTransit,
			expectedResult: valueobject.PayoutInTransit,
			expectedError:  nil,
		},
		{
			name:           "pending to paid rejected",
			from:           valueobject.PayoutPending,
			to:             valueobject.PayoutPaid,
			expectedResult: valueobject.PayoutPending,
			expectedError:  &entity.Error{Code: "PAYOUT_TRANSITION_ILLEGAL", Message: "illegal payout transition"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.TransitionPayout(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestIsOverdue(t *testing.T) {
	t.Parallel()

	submitted := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	expected := submitted.Add(48 * time.Hour)

	type testCase struct {
		name           string
		expectedAt     time.Time
		actualAt       time.Time
		grace          time.Duration
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "past grace",
			expectedAt:     expected,
			actualAt:       expected.Add(2 * time.Hour),
			grace:          time.Hour,
			expectedResult: true,
		},
		{
			name:           "within grace",
			expectedAt:     expected,
			actualAt:       expected.Add(30 * time.Minute),
			grace:          time.Hour,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.IsOverdue(tc.expectedAt, tc.actualAt, tc.grace)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestExpectedSettlementAt(t *testing.T) {
	t.Parallel()

	submitted := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		method         valueobject.PayoutMethod
		submittedAt    time.Time
		expectedResult time.Time
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ach lag",
			method:         valueobject.PayoutACH,
			submittedAt:    submitted,
			expectedResult: submitted.Add(48 * time.Hour),
			expectedError:  nil,
		},
		{
			name:           "unknown rail",
			method:         "NOPE",
			submittedAt:    submitted,
			expectedResult: time.Time{},
			expectedError:  &entity.Error{Code: "PAYOUT_METHOD_UNKNOWN", Message: "payout method is unknown"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ExpectedSettlementAt(tc.method, tc.submittedAt)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
