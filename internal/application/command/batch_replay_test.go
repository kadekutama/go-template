package command_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

func TestBatchIntakeReplay(t *testing.T) {
	t.Parallel()

	t.Run("duplicate intake replays without re-executing", func(t *testing.T) {
		uow := &tfrUOW{}
		store := &tfrStore{}
		authz := &tfrAuthz{denied: map[string]bool{}}
		balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: 90000}}
		svc := newTransferService(uow, store, balances, authz)

		req := port.BatchTransferRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger,
			Items:          []port.TransferRequest{transferTestCommand()},
			IdempotencyKey: "batch-1", Actor: "u-1",
		}
		first, err := svc.CreateBatchTransfer(context.Background(), req)
		require.NoError(t, err)
		second, err := svc.CreateBatchTransfer(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, first, second)
		assert.Len(t, store.batches, 1)
	})

	t.Run("crashed batch converges on resubmission", func(t *testing.T) {
		uow := &tfrUOW{}
		store := &tfrStore{}
		authz := &tfrAuthz{denied: map[string]bool{}}
		balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: 90000}}
		svc := newTransferService(uow, store, balances, authz)

		// Seed a RECEIVED batch whose items never executed (crash after
		// intake commit): batch + items + PENDING intents + completed intake
		// idempotency with a valid intake result.
		batchID := "batch-crash"
		require.NoError(t, store.CreateBatch(context.Background(), command.BatchRecord{
			ID: batchID, TenantID: tfrTenant, LedgerID: tfrLedger, State: command.BatchReceived,
		}, []command.BatchItem{
			{BatchID: batchID, Index: 0, TransferID: "tfr-c0", Status: command.TransferPending},
			{BatchID: batchID, Index: 1, TransferID: "tfr-c1", Status: command.TransferPending},
		}))
		for _, id := range []string{"tfr-c0", "tfr-c1"} {
			require.NoError(t, store.CreateTransfer(context.Background(), command.TransferRecord{
				ID: id, TenantID: tfrTenant, LedgerID: tfrLedger,
				Source: tfrSrc, Dest: tfrDst, AssetCode: tfrAsset, AmountMinor: 5000,
				Status: command.TransferPending,
			}))
		}
		intake := port.BatchTransferResult{BatchID: batchID, TotalItems: 2, AcceptedItems: 2, Cursor: "cursor-5"}
		encoded, err := jsonparser.Marshal(intake)
		require.NoError(t, err)
		// Pins the intake fingerprint contract: key, tenant, ledger, count,
		// then per-item source/dest/asset/amount.
		fingerprint := command.Fingerprint("batch-1", "t-1", "l-1", "2",
			"a-src", "a-dst", "USD", "5000",
			"a-src", "a-dst", "USD", "5000")
		uow.mu.Lock()
		if uow.idem == nil {
			uow.idem = map[string]tfrIdemEntry{}
		}
		uow.idem["batch-1"] = tfrIdemEntry{fingerprint: fingerprint, response: encoded, completed: true}
		uow.mu.Unlock()

		items := []port.TransferRequest{transferTestCommand(), transferTestCommand()}
		resubmitted, err := svc.CreateBatchTransfer(context.Background(), port.BatchTransferRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger, Items: items,
			IdempotencyKey: "batch-1", Actor: "u-1",
		})
		require.NoError(t, err)
		assert.Equal(t, intake, resubmitted)

		querySvc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: store})
		status, err := querySvc.GetBatchStatus(context.Background(), port.BatchStatusQuery{TenantID: tfrTenant, BatchID: batchID})
		require.NoError(t, err)
		assert.Equal(t, command.BatchCompleted, status.State)
		assert.Equal(t, 2, status.Succeeded)
		assert.Contains(t, outboxTypes(uow), command.BatchCompletedEvent)
	})

	t.Run("partially executed batch converges on original indexes", func(t *testing.T) {
		uow := &tfrUOW{}
		store := &tfrStore{}
		authz := &tfrAuthz{denied: map[string]bool{}}
		balances := &tfrBalances{available: map[valueobject.AccountID]int64{tfrSrc: 90000}}
		svc := newTransferService(uow, store, balances, authz)

		// Crash after item 0 completed and item 1 failed, item 2 never ran:
		// convergence must execute only item 2 under its original index key
		// and leave recorded outcomes for 0 and 1 untouched.
		batchID := "batch-partial"
		require.NoError(t, store.CreateBatch(context.Background(), command.BatchRecord{
			ID: batchID, TenantID: tfrTenant, LedgerID: tfrLedger, State: command.BatchReceived,
		}, []command.BatchItem{
			{BatchID: batchID, Index: 0, TransferID: "tfr-p0", Status: command.TransferCompleted},
			{BatchID: batchID, Index: 1, TransferID: "tfr-p1", Status: command.TransferFailed, ErrorCode: "INSUFFICIENT_FUNDS"},
			{BatchID: batchID, Index: 2, TransferID: "tfr-p2", Status: command.TransferPending},
		}))
		for id, status := range map[string]string{
			"tfr-p0": command.TransferCompleted,
			"tfr-p1": command.TransferFailed,
			"tfr-p2": command.TransferPending,
		} {
			require.NoError(t, store.CreateTransfer(context.Background(), command.TransferRecord{
				ID: id, TenantID: tfrTenant, LedgerID: tfrLedger,
				Source: tfrSrc, Dest: tfrDst, AssetCode: tfrAsset, AmountMinor: 5000,
				Status: status,
			}))
		}
		intake := port.BatchTransferResult{BatchID: batchID, TotalItems: 3, AcceptedItems: 3, Cursor: "cursor-5"}
		encoded, err := jsonparser.Marshal(intake)
		require.NoError(t, err)
		fingerprint := command.Fingerprint("batch-1", "t-1", "l-1", "3",
			"a-src", "a-dst", "USD", "5000",
			"a-src", "a-dst", "USD", "5000",
			"a-src", "a-dst", "USD", "5000")
		uow.mu.Lock()
		if uow.idem == nil {
			uow.idem = map[string]tfrIdemEntry{}
		}
		uow.idem["batch-1"] = tfrIdemEntry{fingerprint: fingerprint, response: encoded, completed: true}
		uow.mu.Unlock()

		items := []port.TransferRequest{transferTestCommand(), transferTestCommand(), transferTestCommand()}
		resubmitted, err := svc.CreateBatchTransfer(context.Background(), port.BatchTransferRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger, Items: items,
			IdempotencyKey: "batch-1", Actor: "u-1",
		})
		require.NoError(t, err)
		assert.Equal(t, intake, resubmitted)

		querySvc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: store})
		status, err := querySvc.GetBatchStatus(context.Background(), port.BatchStatusQuery{TenantID: tfrTenant, BatchID: batchID})
		require.NoError(t, err)
		assert.Equal(t, command.BatchPartial, status.State)
		assert.Equal(t, 2, status.Succeeded)
		assert.Equal(t, 1, status.Failed)
		byIndex := map[int]port.BatchItemStatus{}
		for _, item := range status.Items {
			byIndex[item.Index] = item
		}
		assert.Equal(t, command.TransferCompleted, byIndex[0].Status)
		assert.Equal(t, "tfr-p0", byIndex[0].TransferID)
		assert.Equal(t, command.TransferFailed, byIndex[1].Status)
		assert.Equal(t, command.TransferCompleted, byIndex[2].Status)
		assert.Equal(t, "tfr-p2", byIndex[2].TransferID)
	})
}
