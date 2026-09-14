package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestPaymentLinkValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		link          entity.PaymentLink
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid invoice link",
			link:          entity.PaymentLink{PaymentID: "p-1", LinkedType: "invoice", LinkedID: "inv-1"},
			expectedError: nil,
		},
		{
			name:          "valid order link",
			link:          entity.PaymentLink{PaymentID: "p-2", LinkedType: "order", LinkedID: "ord-1"},
			expectedError: nil,
		},
		{
			name:          "valid subscription link",
			link:          entity.PaymentLink{PaymentID: "p-3", LinkedType: "subscription", LinkedID: "sub-1"},
			expectedError: nil,
		},
		{
			name:          "missing payment id",
			link:          entity.PaymentLink{LinkedType: "invoice", LinkedID: "inv-1"},
			expectedError: entity.NewError("PAYMENT_ID_REQUIRED", "payment link requires a payment id"),
		},
		{
			name:          "missing linked type",
			link:          entity.PaymentLink{PaymentID: "p-1", LinkedID: "inv-1"},
			expectedError: entity.NewError("LINK_TARGET_REQUIRED", "payment link requires a linked type and id"),
		},
		{
			name:          "missing linked id",
			link:          entity.PaymentLink{PaymentID: "p-1", LinkedType: "invoice"},
			expectedError: entity.NewError("LINK_TARGET_REQUIRED", "payment link requires a linked type and id"),
		},
		{
			name:          "invalid linked type",
			link:          entity.PaymentLink{PaymentID: "p-1", LinkedType: "chargeback", LinkedID: "cb-1"},
			expectedError: entity.NewError("LINK_TYPE_INVALID", "payment link type must be invoice, order, or subscription"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.link.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestPaymentLinkKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		link           entity.PaymentLink
		expectedResult string
	}

	testCases := []testCase{
		{
			name:           "composite key format",
			link:           entity.PaymentLink{PaymentID: "pay-100", LinkedType: "invoice", LinkedID: "inv-200"},
			expectedResult: "pay-100|invoice|inv-200",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.link.LinkKey()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
