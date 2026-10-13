package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowStoreRequiresDB(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         WorkflowStoreParams
		expectedResult *WorkflowStore
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil DB rejected",
			params: WorkflowStoreParams{
				DB: nil,
			},
			expectedResult: nil,
			expectedError:  ErrPostgresWorkflowStoreNeedsDB,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store, err := NewWorkflowStore(tc.params)
			assert.Equal(t, tc.expectedResult, store)

			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWorkflowStoreTenantClosed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		operation     func() error
		expectedError error
	}

	testCases := []testCase{
		{
			name: "create without tenant",
			operation: func() error {
				store := &WorkflowStore{db: nil}
				_, err := store.Create(context.Background(), WorkflowRecord{ID: "wfl-01"})

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "find without tenant",
			operation: func() error {
				store := &WorkflowStore{db: nil}
				_, err := store.FindByID(context.Background(), "", "wfl-01")

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "status update without tenant",
			operation: func() error {
				store := &WorkflowStore{db: nil}

				return store.UpdateStatus(context.Background(), "", "wfl-01", "DONE", 1)
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "recon source without tenant",
			operation: func() error {
				store := &WorkflowStore{db: nil}
				_, err := store.StoreReconSource(context.Background(), ReconSourceRecord{ID: "src-01", PayloadHash: "h"})

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "recon source without hash",
			operation: func() error {
				store := &WorkflowStore{db: nil}
				_, err := store.StoreReconSource(context.Background(), ReconSourceRecord{ID: "src-01", TenantID: "tnt-01"})

				return err
			},
			expectedError: ErrPostgresPayloadHashIsRequired,
		},
		{
			name: "inbox receipt without tenant",
			operation: func() error {
				store := &WorkflowStore{db: nil}

				return store.InboxReceipt(context.Background(), "", "key-01", "transfer.completed.v1")
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "inbox receipt without key",
			operation: func() error {
				store := &WorkflowStore{db: nil}

				return store.InboxReceipt(context.Background(), "tnt-01", "", "transfer.completed.v1")
			},
			expectedError: ErrPostgresInboxKeyIsRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.operation()
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}

func TestWorkflowRecordMapping(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		record   WorkflowRecord
		expected WorkflowRecord
	}

	testCases := []testCase{
		{
			name: "ledger and posting references round-trip",
			record: WorkflowRecord{
				ID: "wfl-01", TenantID: "tnt-01", LedgerID: "ldg-01",
				Kind: "TRANSFER", Status: "PENDING", PostingID: "pst-01",
				PayloadHash: "hash-01", Version: 1,
			},
			expected: WorkflowRecord{
				ID: "wfl-01", TenantID: "tnt-01", LedgerID: "ldg-01",
				Kind: "TRANSFER", Status: "PENDING", PostingID: "pst-01",
				PayloadHash: "hash-01", Version: 1,
			},
		},
		{
			name: "empty refs stay empty",
			record: WorkflowRecord{
				ID: "wfl-02", TenantID: "tnt-01", Kind: "PAYOUT",
				Status: "PENDING", Version: 1,
			},
			expected: WorkflowRecord{
				ID: "wfl-02", TenantID: "tnt-01", Kind: "PAYOUT",
				Status: "PENDING", Version: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, workflowToRecord(workflowToModel(tc.record)))
		})
	}
}
