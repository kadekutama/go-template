package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestSettlementBatchValidate(t *testing.T) {
	t.Parallel()

	baseBatch := entity.SettlementBatch{
		BatchID:         "b-1",
		ProviderBatchID: "pb-1",
		ProviderTraceID: "pt-1",
		AssetCode:       "USD",
		GrossMinor:      10000,
		FeeMinor:        290,
		NetMinor:        9710,
		CoverageStart:   "2026-09-01T00:00:00Z",
		CoverageEnd:     "2026-09-02T00:00:00Z",
		Items: []entity.SettlementItem{
			{PaymentID: "p-1", Status: entity.SettleItemSettled, AmountMinor: 5000},
			{PaymentID: "p-2", Status: entity.SettleItemPending, AmountMinor: 4710},
		},
	}

	type testCase struct {
		name          string
		batch         entity.SettlementBatch
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid batch",
			batch:         baseBatch,
			expectedError: nil,
		},
		{
			name: "valid batch with failed item status",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{
					{PaymentID: "p-1", Status: entity.SettleItemFailed, AmountMinor: 100},
				}
				return b
			}(),
			expectedError: nil,
		},
		{
			name: "missing batch id",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.BatchID = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_ID_REQUIRED", "settlement batch requires batch and provider batch ids"),
		},
		{
			name: "missing provider batch id",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.ProviderBatchID = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_ID_REQUIRED", "settlement batch requires batch and provider batch ids"),
		},
		{
			name: "missing provider trace id",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.ProviderTraceID = ""
				return b
			}(),
			expectedError: entity.NewError("TRACE_ID_REQUIRED", "settlement batch requires a provider trace id"),
		},
		{
			name: "missing asset code",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.AssetCode = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_ASSET_REQUIRED", "settlement batch requires an asset code"),
		},
		{
			name: "negative gross",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.GrossMinor = -1
				return b
			}(),
			expectedError: entity.NewError("BATCH_TOTAL_INVALID", "settlement batch totals must be non-negative"),
		},
		{
			name: "negative fee",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.FeeMinor = -1
				return b
			}(),
			expectedError: entity.NewError("BATCH_TOTAL_INVALID", "settlement batch totals must be non-negative"),
		},
		{
			name: "negative net",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.NetMinor = -1
				return b
			}(),
			expectedError: entity.NewError("BATCH_TOTAL_INVALID", "settlement batch totals must be non-negative"),
		},
		{
			name: "totals mismatch (gross != fee + net)",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.GrossMinor = 9999
				return b
			}(),
			expectedError: entity.NewError("BATCH_TOTAL_MISMATCH", "settlement batch gross must equal fee plus net"),
		},
		{
			name: "missing coverage start",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.CoverageStart = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_WINDOW_REQUIRED", "settlement batch requires a coverage window"),
		},
		{
			name: "missing coverage end",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.CoverageEnd = ""
				return b
			}(),
			expectedError: entity.NewError("BATCH_WINDOW_REQUIRED", "settlement batch requires a coverage window"),
		},
		{
			name: "nil items",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = nil
				return b
			}(),
			expectedError: entity.NewError("BATCH_ITEMS_REQUIRED", "settlement batch requires at least one item"),
		},
		{
			name: "empty items",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{}
				return b
			}(),
			expectedError: entity.NewError("BATCH_ITEMS_REQUIRED", "settlement batch requires at least one item"),
		},
		{
			name: "item missing payment id",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{
					{PaymentID: "", Status: entity.SettleItemSettled, AmountMinor: 100},
				}
				return b
			}(),
			expectedError: entity.NewError("BATCH_ITEM_PAYMENT_REQUIRED", "settlement item requires a payment id"),
		},
		{
			name: "item zero amount",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{
					{PaymentID: "p-1", Status: entity.SettleItemSettled, AmountMinor: 0},
				}
				return b
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "settlement item amount must be positive"),
		},
		{
			name: "item negative amount",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{
					{PaymentID: "p-1", Status: entity.SettleItemSettled, AmountMinor: -50},
				}
				return b
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "settlement item amount must be positive"),
		},
		{
			name: "item invalid status",
			batch: func() entity.SettlementBatch {
				b := baseBatch
				b.Items = []entity.SettlementItem{
					{PaymentID: "p-1", Status: "UNKNOWN_STATUS", AmountMinor: 100},
				}
				return b
			}(),
			expectedError: entity.NewError("BATCH_ITEM_STATUS_INVALID", "settlement item status is invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.batch.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
