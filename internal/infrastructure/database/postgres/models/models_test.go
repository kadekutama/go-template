package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

func TestLedgerMappingRoundTrip(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ledger        entity.Ledger
		expectedTable string
	}

	testCases := []testCase{
		{
			name: "valid ledger maps and returns",
			ledger: func() entity.Ledger {
				ledger, err := entity.NewLedger("20000000-0000-4000-8000-000000000001", "10000000-0000-4000-8000-000000000001", "Test", "USD", "v1")
				require.NoError(t, err)
				return ledger
			}(),
			expectedTable: "ledgers",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			model := models.LedgerToModel(tc.ledger)
			assert.Equal(t, tc.expectedTable, model.TableName())
			assert.Equal(t, tc.ledger.ID.String(), model.ID)

			back, err := models.LedgerToEntity(model)
			require.NoError(t, err)
			assert.Equal(t, tc.ledger, back)
		})
	}
}

func TestLedgerToEntityRejectsInvalidRow(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name  string
		model models.LedgerModel
	}

	testCases := []testCase{
		{
			name:  "blank id rejected",
			model: models.LedgerModel{TenantID: "10000000-0000-4000-8000-000000000001", Name: "x", BaseAsset: "USD", ChartVersion: "v1"},
		},
		{
			name:  "blank tenant rejected",
			model: models.LedgerModel{ID: "20000000-0000-4000-8000-000000000001", Name: "x", BaseAsset: "USD", ChartVersion: "v1"},
		},
		{
			name:  "blank name rejected",
			model: models.LedgerModel{ID: "20000000-0000-4000-8000-000000000001", TenantID: "10000000-0000-4000-8000-000000000001", BaseAsset: "USD", ChartVersion: "v1"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := models.LedgerToEntity(tc.model)
			require.Error(t, err, "invalid stored rows must surface, never silent zero values")
		})
	}
}

func TestTablePins(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		table         string
		expectedTable string
	}

	testCases := []testCase{
		{
			name:          "accounts pinned",
			table:         models.AccountModel{}.TableName(),
			expectedTable: "accounts",
		},
		{
			name:          "postings pinned",
			table:         models.PostingModel{}.TableName(),
			expectedTable: "postings",
		},
		{
			name:          "entries pinned",
			table:         models.EntryModel{}.TableName(),
			expectedTable: "entries",
		},
		{
			name:          "holds pinned",
			table:         models.HoldModel{}.TableName(),
			expectedTable: "holds",
		},
		{
			name:          "checkpoints pinned",
			table:         models.CheckpointModel{}.TableName(),
			expectedTable: "checkpoints",
		},
		{
			name:          "idempotency pinned",
			table:         models.IdempotencyModel{}.TableName(),
			expectedTable: "idempotency_records",
		},
		{
			name:          "outbox pinned",
			table:         models.OutboxModel{}.TableName(),
			expectedTable: "outbox_events",
		},
		{
			name:          "workflows pinned",
			table:         models.WorkflowModel{}.TableName(),
			expectedTable: "workflows",
		},
		{
			name:          "inbox pinned",
			table:         models.InboxReceiptModel{}.TableName(),
			expectedTable: "inbox_receipts",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedTable, tc.table)
		})
	}
}
