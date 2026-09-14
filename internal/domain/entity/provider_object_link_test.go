package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestProviderObjectLinkValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		link          entity.ProviderObjectLink
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid link",
			link: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "evt-1",
				PaymentID:        "pay-1",
				PostingID:        "pst-1",
			},
			expectedError: nil,
		},
		{
			name: "missing provider object id",
			link: entity.ProviderObjectLink{
				EventID:   "evt-1",
				PaymentID: "pay-1",
				PostingID: "pst-1",
			},
			expectedError: entity.NewError("PROVIDER_OBJECT_REQUIRED", "provider object id is required"),
		},
		{
			name: "missing payment id",
			link: entity.ProviderObjectLink{
				ProviderObjectID: "po-1",
				EventID:          "evt-1",
				PostingID:        "pst-1",
			},
			expectedError: entity.NewError("PAYMENT_ID_REQUIRED", "provider mapping requires a payment id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.link.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestProviderObjectLinkDeliveryKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		link           entity.ProviderObjectLink
		expectedResult string
	}

	testCases := []testCase{
		{
			name: "delivery key format",
			link: entity.ProviderObjectLink{
				ProviderObjectID: "ch_12345",
				EventID:          "evt_67890",
				PaymentID:        "pay-1",
			},
			expectedResult: "ch_12345|evt_67890",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.link.DeliveryKey()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
