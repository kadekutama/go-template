package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateRefundToOriginalMethod(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name          string
		m             valueobject.PaymentMethod
		expectedError error
	}
	testCases := []testCase{
		{
			name:          "ach reversible",
			m:             valueobject.MethodACH,
			expectedError: nil,
		},
		{
			name:          "card reversible",
			m:             valueobject.MethodCard,
			expectedError: nil,
		},
		{
			name:          "wallet reversible",
			m:             valueobject.MethodWallet,
			expectedError: nil,
		},
		{
			name:          "wire irreversible",
			m:             valueobject.MethodWire,
			expectedError: &entity.Error{Code: "METHOD_IRREVERSIBLE", Message: "payment method does not support refund to original method"},
		},
		{
			name:          "rtp irreversible",
			m:             valueobject.MethodRTP,
			expectedError: &entity.Error{Code: "METHOD_IRREVERSIBLE", Message: "payment method does not support refund to original method"},
		},
		{
			name:          "crypto irreversible",
			m:             valueobject.MethodCrypto,
			expectedError: &entity.Error{Code: "METHOD_IRREVERSIBLE", Message: "payment method does not support refund to original method"},
		},
		{
			name:          "unknown method",
			m:             valueobject.PaymentMethod("NOPE"),
			expectedError: &entity.Error{Code: "PAYMENT_METHOD_UNKNOWN", Message: "payment method is unknown"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := service.ValidateRefundToOriginalMethod(tc.m)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransitionPayment(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name           string
		from           valueobject.PaymentStatus
		to             valueobject.PaymentStatus
		expectedResult valueobject.PaymentStatus
		expectedError  error
	}
	testCases := []testCase{
		{
			name:           "authorized to captured",
			from:           valueobject.PaymentAuthorized,
			to:             valueobject.PaymentCaptured,
			expectedResult: valueobject.PaymentCaptured,
			expectedError:  nil,
		},
		{
			name:           "settled to failed rejected",
			from:           valueobject.PaymentSettled,
			to:             valueobject.PaymentFailed,
			expectedResult: valueobject.PaymentSettled,
			expectedError:  &entity.Error{Code: "PAYMENT_TRANSITION_ILLEGAL", Message: "illegal payment transition"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.TransitionPayment(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestDispositionFor(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name           string
		code           string
		expectedResult valueobject.ReturnDisposition
		expectedError  error
	}
	testCases := []testCase{
		{
			name:           "r01",
			code:           "R01",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "r02",
			code:           "R02",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "r03",
			code:           "R03",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r04",
			code:           "R04",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r06",
			code:           "R06",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "r07",
			code:           "R07",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r08",
			code:           "R08",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r09",
			code:           "R09",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "r10",
			code:           "R10",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r11",
			code:           "R11",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r12",
			code:           "R12",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "r13",
			code:           "R13",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r14",
			code:           "R14",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r15",
			code:           "R15",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r16",
			code:           "R16",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "r17",
			code:           "R17",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card declined",
			code:           "CARD_DECLINED",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "card insufficient",
			code:           "CARD_INSUFFICIENT",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "card fraud suspected",
			code:           "CARD_FRAUD_SUSPECTED",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card expired",
			code:           "CARD_EXPIRED",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card invalid",
			code:           "CARD_INVALID",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "unknown code",
			code:           "NOPE",
			expectedResult: valueobject.ReturnDisposition(""),
			expectedError:  &entity.Error{Code: "RETURN_CODE_UNKNOWN", Message: "return code is unknown"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.DispositionFor(tc.code)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestAddLink(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name          string
		existing      map[string]entity.PaymentLink
		link          entity.PaymentLink
		expectedError error
	}
	testCases := []testCase{
		{
			name:     "ok",
			existing: map[string]entity.PaymentLink{},
			link: entity.PaymentLink{
				PaymentID:  "p-1",
				LinkedType: "invoice",
				LinkedID:   "i-1",
			},
			expectedError: nil,
		},
		{
			name: "duplicate",
			existing: map[string]entity.PaymentLink{
				"p-1|invoice|i-1": {
					PaymentID:  "p-1",
					LinkedType: "invoice",
					LinkedID:   "i-1",
				},
			},
			link: entity.PaymentLink{
				PaymentID:  "p-1",
				LinkedType: "invoice",
				LinkedID:   "i-1",
			},
			expectedError: &entity.Error{Code: "DUPLICATE_LINK", Message: "payment link already exists"},
		},
		{
			name:          "invalid link",
			existing:      map[string]entity.PaymentLink{},
			link:          entity.PaymentLink{},
			expectedError: &entity.Error{Code: "PAYMENT_ID_REQUIRED", Message: "payment link requires a payment id"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := service.AddLink(tc.existing, tc.link)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestResolveTimeout(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name           string
		expectedResult service.ProviderOutcome
	}
	testCases := []testCase{
		{
			name:           "timeout is unknown",
			expectedResult: service.OutcomeUnknown,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := service.ResolveTimeout()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestRequireStatusLookup(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name            string
		outcome         service.ProviderOutcome
		lookupConfirmed bool
		expectedError   error
	}
	testCases := []testCase{
		{
			name:            "blind retry rejected",
			outcome:         service.OutcomeUnknown,
			lookupConfirmed: false,
			expectedError:   &entity.Error{Code: "OUTCOME_UNKNOWN", Message: "provider outcome unknown: status lookup required before retry"},
		},
		{
			name:            "lookup clears retry",
			outcome:         service.OutcomeUnknown,
			lookupConfirmed: true,
			expectedError:   nil,
		},
		{
			name:            "confirmed needs no lookup",
			outcome:         service.OutcomeConfirmed,
			lookupConfirmed: false,
			expectedError:   nil,
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := service.RequireStatusLookup(tc.outcome, tc.lookupConfirmed)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestDuplicateDelivery(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name            string
		existing        map[string]entity.ProviderObjectLink
		link            entity.ProviderObjectLink
		expectedResult1 entity.ProviderObjectLink
		expectedResult2 bool
		expectedError   error
	}
	testCases := []testCase{
		{
			name:     "first delivery",
			existing: map[string]entity.ProviderObjectLink{},
			link: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "e-1",
				PaymentID:        "p-1",
				PostingID:        "pst-1",
			},
			expectedResult1: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "e-1",
				PaymentID:        "p-1",
				PostingID:        "pst-1",
			},
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name: "replay returns existing",
			existing: map[string]entity.ProviderObjectLink{
				"po-1|e-1": {
					ProviderObjectID: "po-1",
					EventID:          "e-1",
					PaymentID:        "p-1",
					PostingID:        "pst-1",
				},
			},
			link: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "e-1",
				PaymentID:        "p-1",
				PostingID:        "pst-1",
			},
			expectedResult1: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "e-1",
				PaymentID:        "p-1",
				PostingID:        "pst-1",
			},
			expectedResult2: true,
			expectedError:   nil,
		},
		{
			name:            "invalid link",
			existing:        map[string]entity.ProviderObjectLink{},
			link:            entity.ProviderObjectLink{},
			expectedResult1: entity.ProviderObjectLink{},
			expectedResult2: false,
			expectedError:   &entity.Error{Code: "PROVIDER_OBJECT_REQUIRED", Message: "provider object id is required"},
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult1, actualResult2, err := service.DuplicateDelivery(tc.existing, tc.link)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
