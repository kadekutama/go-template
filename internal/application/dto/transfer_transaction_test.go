package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/application/port"
)

func TestToTransferDTO(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	future := at.Add(24 * time.Hour)

	type testCase struct {
		name           string
		record         command.TransferRecord
		cursor         string
		expectedResult dto.TransferDTO
	}

	testCases := []testCase{
		{
			name: "immediate transfer maps without schedule",
			record: command.TransferRecord{
				ID: "x-1", TenantID: "t-1", LedgerID: "l-1", Source: "a-src", Dest: "a-dst",
				AssetCode: "USD", AmountMinor: 5000, Status: "COMPLETED",
				PostingID: "p-1", CreatedAt: at,
			},
			cursor: "cursor-5",
			expectedResult: dto.TransferDTO{
				ID: "x-1", TenantID: "t-1", LedgerID: "l-1", Source: "a-src", Dest: "a-dst",
				AssetCode: "USD", AmountMinor: 5000, Status: "COMPLETED",
				PostingID: "p-1", CreatedAt: at, Cursor: "cursor-5",
			},
		},
		{
			name: "scheduled transfer maps execute time",
			record: command.TransferRecord{
				ID: "x-2", TenantID: "t-1", LedgerID: "l-1", Source: "a-src", Dest: "a-dst",
				AssetCode: "USD", AmountMinor: 5000, Status: "PENDING",
				ExecuteAt: future, Recurrence: "monthly", CreatedAt: at,
			},
			cursor: "cursor-5",
			expectedResult: dto.TransferDTO{
				ID: "x-2", TenantID: "t-1", LedgerID: "l-1", Source: "a-src", Dest: "a-dst",
				AssetCode: "USD", AmountMinor: 5000, Status: "PENDING",
				ExecuteAt: &future, Recurrence: "monthly", CreatedAt: at, Cursor: "cursor-5",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToTransferDTO(tc.record, tc.cursor))
		})
	}
}

func TestToBatchDTO(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	t.Run("batch status maps counts and items", func(t *testing.T) {
		actual := dto.ToBatchStatusDTO(port.BatchStatusResult{
			BatchID: "b-1", State: "PARTIAL", Succeeded: 2, Failed: 1,
			Items: []port.BatchItemStatus{{Index: 1, TransferID: "x-2", Status: "FAILED", ErrorCode: "INSUFFICIENT_FUNDS"}},
		})
		assert.Equal(t, "b-1", actual.ID)
		assert.Equal(t, "PARTIAL", actual.State)
		assert.Equal(t, 1, actual.TotalItems)
		assert.Equal(t, 2, actual.Succeeded)
		assert.Equal(t, 1, actual.Failed)
		assert.Equal(t, "INSUFFICIENT_FUNDS", actual.Items[0].ErrorCode)
	})

	t.Run("stored batch maps record plus outcomes", func(t *testing.T) {
		actual := dto.ToBatchDTO(command.BatchRecord{
			ID: "b-1", TenantID: "t-1", LedgerID: "l-1", State: "COMPLETED", CreatedAt: at,
		}, []command.BatchItem{
			{BatchID: "b-1", Index: 0, TransferID: "x-1", Status: "COMPLETED"},
		}, 1, 0, "cursor-5")
		assert.Equal(t, "b-1", actual.ID)
		assert.Equal(t, "COMPLETED", actual.State)
		assert.Equal(t, at, actual.CreatedAt)
		assert.Equal(t, "cursor-5", actual.Cursor)
	})
}

func TestToReverseTransactionDTO(t *testing.T) {
	t.Parallel()

	t.Run("reversal maps link and cursor", func(t *testing.T) {
		actual := dto.ToReverseTransactionDTO(port.PostingResult{
			PostingID: "p-2", TenantID: "t-1", LedgerID: "l-1", Cursor: "cursor-5",
		}, "p-1", "duplicate")
		assert.Equal(t, "p-2", actual.ID)
		assert.Equal(t, "p-1", actual.OriginalID)
		assert.Equal(t, "duplicate", actual.Reason)
		assert.Equal(t, "cursor-5", actual.Cursor)
	})
}
