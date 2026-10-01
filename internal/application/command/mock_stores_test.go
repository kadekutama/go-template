package command_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

func newMockSubscriptionStore(t *testing.T) *mockcommand.MockSubscriptionStore {
	t.Helper()
	store := mockcommand.NewMockSubscriptionStore(t)
	var subs sync.Map

	store.EXPECT().CreateSubscription(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, sub command.Subscription) error {
		subs.Store(sub.ID, sub)
		return nil
	}).Maybe()

	store.EXPECT().FindSubscription(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.Subscription, error) {
		val, ok := subs.Load(id)
		if !ok {
			return command.Subscription{}, entity.NewError("SUBSCRIPTION_NOT_FOUND", "subscription is unknown")
		}
		return val.(command.Subscription), nil
	}).Maybe()

	store.EXPECT().UpdateSubscription(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, sub command.Subscription) error {
		subs.Store(sub.ID, sub)
		return nil
	}).Maybe()

	store.EXPECT().DeleteSubscription(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) error {
		subs.Delete(id)
		return nil
	}).Maybe()

	store.EXPECT().ListDueSubscriptions(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, now time.Time) ([]command.Subscription, error) {
		var out []command.Subscription
		subs.Range(func(_, value any) bool {
			sub := value.(command.Subscription)
			if !sub.LastRun.After(now.Add(-24 * time.Hour)) {
				out = append(out, sub)
			}
			return true
		})
		return out, nil
	}).Maybe()

	return store
}

func newMockReportStore(t *testing.T) *mockcommand.MockReportStore {
	t.Helper()
	store := mockcommand.NewMockReportStore(t)
	var reports sync.Map

	store.EXPECT().CreateReport(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.ReportRecord) error {
		reports.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().FindReport(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.ReportRecord, error) {
		val, ok := reports.Load(id)
		if !ok {
			return command.ReportRecord{}, entity.NewError("REPORT_NOT_FOUND", "report is unknown")
		}
		return val.(command.ReportRecord), nil
	}).Maybe()

	store.EXPECT().ListReports(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.ReportRecord, error) {
		var out []command.ReportRecord
		reports.Range(func(_, value any) bool {
			out = append(out, value.(command.ReportRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	store.EXPECT().UpdateReport(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.ReportRecord) error {
		reports.Store(record.ID, record)
		return nil
	}).Maybe()

	return store
}

func newMockObjectStorage(t *testing.T) *mockapplication.MockObjectStorage {
	t.Helper()
	storage := mockapplication.NewMockObjectStorage(t)
	var objects sync.Map

	storage.EXPECT().Put(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, key string, object port.StoredObject) error {
		objects.Store(key, object.Content)
		return nil
	}).Maybe()

	storage.EXPECT().Get(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, key string) (port.StoredObject, error) {
		val, ok := objects.Load(key)
		if !ok {
			return port.StoredObject{}, entity.NewError("OBJECT_NOT_FOUND", "object is unknown")
		}
		return port.StoredObject{Content: val.([]byte)}, nil
	}).Maybe()

	storage.EXPECT().Delete(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, key string) error {
		objects.Delete(key)
		return nil
	}).Maybe()

	storage.EXPECT().SignedURL(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, key string, _ time.Duration) (string, error) {
		return "https://cdn.example/" + key, nil
	}).Maybe()

	return storage
}

func newMockComplianceStore(t *testing.T) *mockcommand.MockComplianceStore {
	t.Helper()
	store := mockcommand.NewMockComplianceStore(t)
	var reviews sync.Map
	var exports sync.Map

	store.EXPECT().RecordDecision(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, decision command.ScreeningDecision) error {
		reviews.Store(decision.ID, decision)
		return nil
	}).Maybe()

	store.EXPECT().SaveExport(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, export command.RegulatoryExport) error {
		exports.Store(export.ID, export)
		return nil
	}).Maybe()

	store.EXPECT().FindExport(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.RegulatoryExport, error) {
		val, ok := exports.Load(id)
		if !ok {
			return command.RegulatoryExport{}, entity.NewError("EXPORT_NOT_FOUND", "export is unknown")
		}
		return val.(command.RegulatoryExport), nil
	}).Maybe()

	return store
}

func newMockPeriodStore(t *testing.T) *mockcommand.MockPeriodStore {
	t.Helper()
	store := mockcommand.NewMockPeriodStore(t)
	var periods sync.Map

	store.EXPECT().FindPeriod(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _, _, id string) (entity.PeriodData, error) {
		val, ok := periods.Load(id)
		if !ok {
			return entity.PeriodData{}, entity.NewError("PERIOD_NOT_FOUND", "period is unknown")
		}
		return val.(entity.PeriodData), nil
	}).Maybe()

	store.EXPECT().ListPeriods(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _, _ string, limit int) ([]entity.PeriodData, error) {
		var out []entity.PeriodData
		periods.Range(func(_, value any) bool {
			out = append(out, value.(entity.PeriodData))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	store.EXPECT().SavePeriod(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, period entity.PeriodData) error {
		periods.Store(string(period.ID), period)
		return nil
	}).Maybe()

	return store
}

func newMockReconStore(t *testing.T) *mockcommand.MockReconStore {
	t.Helper()
	return newMockReconStoreWithOpenBreaks(t, 0)
}

func newMockReconStoreWithOpenBreaks(t *testing.T, openN int) *mockcommand.MockReconStore {
	t.Helper()
	store := mockcommand.NewMockReconStore(t)
	var runs sync.Map
	var breaks sync.Map
	var openCount atomic.Int64
	openCount.Store(int64(openN))

	store.EXPECT().CreateRun(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, run command.ReconRunRecord) error {
		runs.Store(run.ID, run)
		return nil
	}).Maybe()

	store.EXPECT().FindRun(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.ReconRunRecord, error) {
		val, ok := runs.Load(id)
		if !ok {
			return command.ReconRunRecord{}, entity.NewError("RUN_NOT_FOUND", "reconciliation run is unknown")
		}
		return val.(command.ReconRunRecord), nil
	}).Maybe()

	store.EXPECT().UpdateRun(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, run command.ReconRunRecord) error {
		runs.Store(run.ID, run)
		return nil
	}).Maybe()

	store.EXPECT().CreateBreaks(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, items []command.BreakRecord) error {
		for _, item := range items {
			breaks.Store(item.Break.BreakID, item)
		}
		return nil
	}).Maybe()

	store.EXPECT().FindBreak(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.BreakRecord, error) {
		val, ok := breaks.Load(id)
		if !ok {
			return command.BreakRecord{}, entity.NewError("BREAK_NOT_FOUND", "break is unknown")
		}
		return val.(command.BreakRecord), nil
	}).Maybe()

	store.EXPECT().ListBreaksByRun(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, runID string) ([]command.BreakRecord, error) {
		var out []command.BreakRecord
		breaks.Range(func(_, value any) bool {
			b := value.(command.BreakRecord)
			if b.Break.RunID == runID {
				out = append(out, b)
			}
			return true
		})
		return out, nil
	}).Maybe()

	store.EXPECT().UpdateBreak(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, item command.BreakRecord) error {
		breaks.Store(item.Break.BreakID, item)
		return nil
	}).Maybe()

	store.EXPECT().CountOpenBreaks(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, _ valueobject.LedgerID) (int, error) {
		return int(openCount.Load()), nil
	}).Maybe()

	store.EXPECT().ListRuns(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.ReconRunRecord, error) {
		var out []command.ReconRunRecord
		runs.Range(func(_, value any) bool {
			out = append(out, value.(command.ReconRunRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	store.EXPECT().ListBreaks(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, status string, limit int) ([]command.BreakRecord, error) {
		var out []command.BreakRecord
		breaks.Range(func(_, value any) bool {
			b := value.(command.BreakRecord)
			if status == "" || string(b.Status) == status {
				out = append(out, b)
			}
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	return store
}

func newMockPayoutStore(t *testing.T, initialPolicies ...valueobject.PayoutPolicy) *mockcommand.MockPayoutStore {
	t.Helper()
	var initialPolicy valueobject.PayoutPolicy
	if len(initialPolicies) > 0 {
		initialPolicy = initialPolicies[0]
	}
	store := mockcommand.NewMockPayoutStore(t)
	var payouts sync.Map
	var policy atomic.Pointer[valueobject.PayoutPolicy]
	if initialPolicy.TenantID != "" || initialPolicy.AssetCode != "" {
		policy.Store(&initialPolicy)
	}

	store.EXPECT().CreatePayout(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.PayoutRecord) error {
		payouts.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().FindPayout(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.PayoutRecord, error) {
		val, ok := payouts.Load(id)
		if !ok {
			return command.PayoutRecord{}, entity.NewError("PAYOUT_NOT_FOUND", "payout is unknown")
		}
		return val.(command.PayoutRecord), nil
	}).Maybe()

	store.EXPECT().UpdatePayout(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.PayoutRecord) error {
		payouts.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().GetPolicy(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, _ valueobject.AssetCode) (valueobject.PayoutPolicy, error) {
		p := policy.Load()
		if p == nil {
			return valueobject.PayoutPolicy{}, nil
		}
		return *p, nil
	}).Maybe()

	store.EXPECT().UpdatePolicy(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p valueobject.PayoutPolicy) error {
		policy.Store(&p)
		return nil
	}).Maybe()

	store.EXPECT().ListPayouts(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.PayoutRecord, error) {
		var out []command.PayoutRecord
		payouts.Range(func(_, value any) bool {
			out = append(out, value.(command.PayoutRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	return store
}

func newMockTopupStore(t *testing.T) *mockcommand.MockTopupStore {
	t.Helper()
	store := mockcommand.NewMockTopupStore(t)
	var topups sync.Map

	store.EXPECT().CreateTopup(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.TopupRecord) error {
		topups.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().FindTopup(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.TopupRecord, error) {
		val, ok := topups.Load(id)
		if !ok {
			return command.TopupRecord{}, entity.NewError("TOPUP_NOT_FOUND", "top-up is unknown")
		}
		return val.(command.TopupRecord), nil
	}).Maybe()

	store.EXPECT().UpdateTopup(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.TopupRecord) error {
		topups.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().ListTopups(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.TopupRecord, error) {
		var out []command.TopupRecord
		topups.Range(func(_, value any) bool {
			out = append(out, value.(command.TopupRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	return store
}

func newMockRefundStore(t *testing.T) *mockcommand.MockRefundStore {
	t.Helper()
	store := mockcommand.NewMockRefundStore(t)
	var refunds sync.Map

	store.EXPECT().CreateRefund(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.RefundRecord) error {
		refunds.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().FindRefund(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.RefundRecord, error) {
		val, ok := refunds.Load(id)
		if !ok {
			return command.RefundRecord{}, entity.NewError("REFUND_NOT_FOUND", "refund is unknown")
		}
		return val.(command.RefundRecord), nil
	}).Maybe()

	store.EXPECT().SumPriorRefunds(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, originalTxn string) (int64, error) {
		var sum int64
		refunds.Range(func(_, value any) bool {
			r := value.(command.RefundRecord)
			if r.OriginalTxn == originalTxn && r.Status == command.RefundSucceeded {
				sum += r.AmountMinor
			}
			return true
		})
		return sum, nil
	}).Maybe()

	store.EXPECT().ListRefunds(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.RefundRecord, error) {
		var out []command.RefundRecord
		refunds.Range(func(_, value any) bool {
			out = append(out, value.(command.RefundRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	return store
}

func newMockDisputeStore(t *testing.T) *mockcommand.MockDisputeStore {
	t.Helper()
	store := mockcommand.NewMockDisputeStore(t)
	var disputes sync.Map

	store.EXPECT().CreateDispute(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, dispute entity.Dispute) error {
		disputes.Store(dispute.ID, dispute)
		return nil
	}).Maybe()

	store.EXPECT().FindDispute(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, id string) (entity.Dispute, error) {
		val, ok := disputes.Load(id)
		if !ok {
			return entity.Dispute{}, entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown")
		}
		return val.(entity.Dispute), nil
	}).Maybe()

	store.EXPECT().UpdateDispute(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, dispute entity.Dispute) error {
		disputes.Store(dispute.ID, dispute)
		return nil
	}).Maybe()

	store.EXPECT().ListDisputes(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, status string, from, to time.Time, _ string, limit int) ([]entity.Dispute, string, error) {
		var out []entity.Dispute
		disputes.Range(func(_, value any) bool {
			d := value.(entity.Dispute)
			if status != "" && string(d.Status) != status {
				return true
			}
			if !from.IsZero() && d.OpenedAt.Before(from) {
				return true
			}
			if !to.IsZero() && !d.OpenedAt.Before(to) {
				return true
			}
			out = append(out, d)
			return true
		})
		return out, "", nil
	}).Maybe()

	return store
}

func newMockIntentStore(t *testing.T) *mockcommand.MockIntentStore {
	t.Helper()
	store := mockcommand.NewMockIntentStore(t)
	var intents sync.Map

	store.EXPECT().CreateIntent(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.IntentRecord) error {
		intents.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().FindIntent(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.IntentRecord, error) {
		val, ok := intents.Load(id)
		if !ok {
			return command.IntentRecord{}, entity.NewError("INTENT_NOT_FOUND", "intent is unknown")
		}
		return val.(command.IntentRecord), nil
	}).Maybe()

	store.EXPECT().UpdateIntent(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.IntentRecord) error {
		intents.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().ListIntents(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, limit int) ([]command.IntentRecord, error) {
		var out []command.IntentRecord
		intents.Range(func(_, value any) bool {
			out = append(out, value.(command.IntentRecord))
			return limit <= 0 || len(out) < limit
		})
		return out, nil
	}).Maybe()

	return store
}

type transferStoreStats struct {
	transfers atomic.Int64
	batches   atomic.Int64
}

var transferStoreStatsMap sync.Map

func transferCount(m *mockcommand.MockTransferStore) int {
	if v, ok := transferStoreStatsMap.Load(m); ok {
		return int(v.(*transferStoreStats).transfers.Load())
	}
	return 0
}

func batchCount(m *mockcommand.MockTransferStore) int {
	if v, ok := transferStoreStatsMap.Load(m); ok {
		return int(v.(*transferStoreStats).batches.Load())
	}
	return 0
}

func newMockTransferStore(t *testing.T) *mockcommand.MockTransferStore {
	t.Helper()
	store := mockcommand.NewMockTransferStore(t)
	var transfers sync.Map
	var batches sync.Map
	var batchItems sync.Map
	stats := &transferStoreStats{}
	transferStoreStatsMap.Store(store, stats)

	store.EXPECT().CreateTransfer(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.TransferRecord) error {
		if _, dup := transfers.LoadOrStore(record.ID, record); dup {
			return entity.NewError("TRANSFER_CONFLICT", "transfer id already exists")
		}
		stats.transfers.Add(1)
		return nil
	}).Maybe()

	store.EXPECT().FindTransfer(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.TransferRecord, error) {
		val, ok := transfers.Load(id)
		if !ok {
			return command.TransferRecord{}, entity.NewError("TRANSFER_NOT_FOUND", "transfer is unknown")
		}
		return val.(command.TransferRecord), nil
	}).Maybe()

	store.EXPECT().UpdateTransfer(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, record command.TransferRecord) error {
		transfers.Store(record.ID, record)
		return nil
	}).Maybe()

	store.EXPECT().ListTransfers(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, filter command.TransferListFilter) ([]command.TransferRecord, string, error) {
		var out []command.TransferRecord
		transfers.Range(func(_, value any) bool {
			record := value.(command.TransferRecord)
			if record.TenantID != filter.TenantID {
				return true
			}
			if filter.Status != "" && record.Status != filter.Status {
				return true
			}
			out = append(out, record)
			return true
		})
		return out, "", nil
	}).Maybe()

	store.EXPECT().CreateBatch(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, batch command.BatchRecord, items []command.BatchItem) error {
		batches.Store(batch.ID, batch)
		batchItems.Store(batch.ID, append([]command.BatchItem(nil), items...))
		stats.batches.Add(1)
		return nil
	}).Maybe()

	store.EXPECT().FindBatch(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id string) (command.BatchRecord, error) {
		val, ok := batches.Load(id)
		if !ok {
			return command.BatchRecord{}, entity.NewError("BATCH_NOT_FOUND", "batch is unknown")
		}
		return val.(command.BatchRecord), nil
	}).Maybe()

	store.EXPECT().UpdateBatchState(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id, state string) error {
		val, ok := batches.Load(id)
		if !ok {
			return entity.NewError("BATCH_NOT_FOUND", "batch is unknown")
		}
		b := val.(command.BatchRecord)
		b.State = state
		batches.Store(id, b)
		return nil
	}).Maybe()

	store.EXPECT().UpdateBatchItem(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, batchID string, index int, status, errorCode string) error {
		val, ok := batchItems.Load(batchID)
		if !ok {
			return nil
		}
		items := val.([]command.BatchItem)
		if index >= 0 && index < len(items) {
			items[index].Status = status
			items[index].ErrorCode = errorCode
			batchItems.Store(batchID, items)
		}
		return nil
	}).Maybe()

	store.EXPECT().ListBatchItems(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, batchID string) ([]command.BatchItem, error) {
		val, ok := batchItems.Load(batchID)
		if !ok {
			return nil, nil
		}
		items := val.([]command.BatchItem)
		return append([]command.BatchItem(nil), items...), nil
	}).Maybe()

	return store
}

type mockProcessorParams struct {
	ChargeResult    port.ChargeResult
	ChargeErr       error
	OnCharge        func()
	RefundErr       error
	ChallengeResult port.ChargeResult
	ChallengeErr    error
}

func newMockPaymentProcessor(t *testing.T, opts ...mockProcessorParams) *mockapplication.MockPaymentProcessor {
	t.Helper()
	m := mockapplication.NewMockPaymentProcessor(t)
	var opt mockProcessorParams
	if len(opts) > 0 {
		opt = opts[0]
	}

	m.EXPECT().Charge(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ port.ChargeRequest) (port.ChargeResult, error) {
		if opt.OnCharge != nil {
			opt.OnCharge()
		}
		if opt.ChargeErr != nil {
			return port.ChargeResult{}, opt.ChargeErr
		}
		return opt.ChargeResult, nil
	}).Maybe()

	m.EXPECT().RefundCharge(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ port.RefundChargeRequest) (port.ChargeResult, error) {
		if opt.RefundErr != nil {
			return port.ChargeResult{}, opt.RefundErr
		}
		return port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED", TraceID: "trace-1"}, nil
	}).Maybe()

	m.EXPECT().GetStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, _ string, _ time.Duration) (port.ChargeResult, error) {
		return opt.ChargeResult, opt.ChargeErr
	}).Maybe()

	m.EXPECT().CompleteChallenge(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ port.ChallengeCompletion) (port.ChargeResult, error) {
		if opt.ChallengeErr != nil {
			return port.ChargeResult{}, opt.ChallengeErr
		}
		return opt.ChallengeResult, nil
	}).Maybe()

	return m
}

func newMockClock(t *testing.T, now ...time.Time) *mockapplication.MockClock {
	var clock *mockapplication.MockClock
	if t != nil {
		t.Helper()
		clock = mockapplication.NewMockClock(t)
	} else {
		clock = &mockapplication.MockClock{}
	}
	target := time.Now()
	if len(now) > 0 && !now[0].IsZero() {
		target = now[0]
	}
	clock.EXPECT().Now().Return(target).Maybe()
	return clock
}

func newMockIdempotencyStore(t *testing.T) *mockapplication.MockIdempotencyStore {
	t.Helper()
	store := mockapplication.NewMockIdempotencyStore(t)
	var entries sync.Map

	type idemEntry struct {
		fingerprint string
		response    []byte
		completed   bool
	}

	store.EXPECT().Reserve(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
		val, ok := entries.Load(rec.Key)
		if !ok {
			entries.Store(rec.Key, idemEntry{fingerprint: rec.Fingerprint})
			return port.ReserveOutcome{}, nil
		}
		entry := val.(idemEntry)
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		if entry.completed {
			return port.ReserveOutcome{Replay: true, Response: entry.response}, nil
		}
		return port.ReserveOutcome{}, nil
	}).Maybe()

	store.EXPECT().Complete(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, response []byte) error {
		val, ok := entries.Load(key)
		var entry idemEntry
		if ok {
			entry = val.(idemEntry)
		}
		entry.response = response
		entry.completed = true
		entries.Store(key, entry)
		return nil
	}).Maybe()

	return store
}

func newMockAccountRepository(t *testing.T, accounts map[valueobject.AccountID]entity.AccountData) *mockdomain.MockAccountRepository {
	var repo *mockdomain.MockAccountRepository
	if t != nil {
		t.Helper()
		repo = mockdomain.NewMockAccountRepository(t)
	} else {
		repo = &mockdomain.MockAccountRepository{}
	}
	var accts sync.Map
	for k, v := range accounts {
		accts.Store(k, v)
	}

	repo.EXPECT().Create(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, acct entity.AccountData) (entity.AccountData, error) {
		accts.Store(acct.ID, acct)
		return acct, nil
	}).Maybe()

	repo.EXPECT().FindByID(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error) {
		val, ok := accts.Load(id)
		if !ok {
			return entity.AccountData{}, entity.NewError("ACCOUNT_NOT_FOUND", "account "+string(id)+" is unknown")
		}
		return val.(entity.AccountData), nil
	}).Maybe()

	repo.EXPECT().FindByTenant(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
		return nil, "", nil
	}).Maybe()

	repo.EXPECT().UpdateMetadata(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	repo.EXPECT().UpdateStatus(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	return repo
}

func newMockBalances(t *testing.T, available map[valueobject.AccountID]int64) *mockapplication.MockGetBalance {
	var bal *mockapplication.MockGetBalance
	if t != nil {
		t.Helper()
		bal = mockapplication.NewMockGetBalance(t)
	} else {
		bal = &mockapplication.MockGetBalance{}
	}
	bal.EXPECT().Execute(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, q port.BalanceQuery) (port.BalanceView, error) {
		var avail int64
		if available != nil {
			avail = available[q.AccountID]
		}
		return port.BalanceView{
			AccountID:      q.AccountID,
			AssetCode:      q.AssetCode,
			AvailableMinor: avail,
			Cursor:         "cursor-b",
		}, nil
	}).Maybe()
	return bal
}

type authzStats struct {
	calls  atomic.Int64
	denied sync.Map
}

var authzStatsMap sync.Map

func newMockAuthorizer(t *testing.T) *mockapplication.MockAuthorizer {
	var authz *mockapplication.MockAuthorizer
	if t != nil {
		t.Helper()
		authz = mockapplication.NewMockAuthorizer(t)
	} else {
		authz = &mockapplication.MockAuthorizer{}
	}
	stats := &authzStats{}
	authzStatsMap.Store(authz, stats)

	authz.EXPECT().Authorize(mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, subject port.Subject, action, resource string) error {
		stats.calls.Add(1)
		if v, ok := stats.denied.Load(subject.ID + "|" + action + "|" + resource); ok && v.(bool) {
			return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
		}
		return nil
	}).Maybe()

	return authz
}

func authzCalls(a port.Authorizer) int {
	if v, ok := authzStatsMap.Load(a); ok {
		return int(v.(*authzStats).calls.Load())
	}
	return 0
}

func setAuthzDenied(a port.Authorizer, key string) {
	if v, ok := authzStatsMap.Load(a); ok {
		v.(*authzStats).denied.Store(key, true)
	}
}

func newMockIDGenerator(t *testing.T, ids ...string) *mockapplication.MockIDGenerator {
	var m *mockapplication.MockIDGenerator
	if t != nil {
		t.Helper()
		m = mockapplication.NewMockIDGenerator(t)
	} else {
		m = &mockapplication.MockIDGenerator{}
	}
	var counter atomic.Int64
	m.EXPECT().NewID().RunAndReturn(func() string {
		idx := int(counter.Add(1)) - 1
		if idx < len(ids) {
			return ids[idx]
		}
		return fmt.Sprintf("90000000-0000-4000-8000-%012d", idx+1)
	}).Maybe()
	return m
}

func tfrTestIDs(n int) []string {
	ids := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ids = append(ids, fmt.Sprintf("tfr-%02d", i))
	}
	return ids
}

func postTestIDs(n int) []string {
	ids := make([]string, 0, n)
	for i := 1; i <= n; i++ {
		ids = append(ids, fmt.Sprintf("90000000-0000-4000-8000-%012d", i))
	}
	return ids
}

func newMockPostingRepository(t *testing.T, original entity.PostingData, findErr error) *mockdomain.MockPostingRepository {
	var repo *mockdomain.MockPostingRepository
	if t != nil {
		t.Helper()
		repo = mockdomain.NewMockPostingRepository(t)
	} else {
		repo = &mockdomain.MockPostingRepository{}
	}
	repo.EXPECT().Commit(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, p entity.PostingData) (entity.PostingData, error) {
		return p, nil
	}).Maybe()
	repo.EXPECT().FindByID(mock.Anything, mock.Anything, mock.Anything).Return(original, findErr).Maybe()
	repo.EXPECT().FindByExternalReference(mock.Anything, mock.Anything, mock.Anything).Return(entity.PostingData{}, findErr).Maybe()
	repo.EXPECT().FindByAccount(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, "", findErr).Maybe()
	return repo
}

type mockUOWTracker struct {
	FailAfterCommit atomic.Bool
	commits         atomic.Int64
	outbox          atomic.Pointer[[]port.OutboxFact]
	idem            sync.Map
	postings        sync.Map
	postingsCount   atomic.Int64
}

type mockUOWIdemEntry struct {
	fingerprint string
	response    []byte
	completed   atomic.Bool
	inFlight    atomic.Pointer[chan struct{}]
}

func (tr *mockUOWTracker) OutboxFacts() []port.OutboxFact {
	if o := tr.outbox.Load(); o != nil {
		return *o
	}
	return nil
}

func (tr *mockUOWTracker) OutboxLen() int {
	return len(tr.OutboxFacts())
}

func (tr *mockUOWTracker) CommitCount() int {
	return int(tr.commits.Load())
}

func (tr *mockUOWTracker) SetIdemEntry(key, fingerprint string, response []byte) {
	entry := &mockUOWIdemEntry{
		fingerprint: fingerprint,
		response:    response,
	}
	entry.completed.Store(true)
	tr.idem.Store(key, entry)
}

var uowTrackerMap sync.Map

func uowTracker(uow port.UnitOfWork) *mockUOWTracker {
	if m, ok := uow.(*mockapplication.MockUnitOfWork); ok {
		if tr, ok := uowTrackerMap.Load(m); ok {
			return tr.(*mockUOWTracker)
		}
	}
	return nil
}

func outboxFacts(uow port.UnitOfWork) []port.OutboxFact {
	if tr := uowTracker(uow); tr != nil {
		return tr.OutboxFacts()
	}
	return nil
}

func outboxTypes(uow port.UnitOfWork) []string {
	tr := uowTracker(uow)
	if tr == nil {
		return nil
	}
	var types []string
	for _, f := range tr.OutboxFacts() {
		types = append(types, f.EventType)
	}
	return types
}

func outboxPayload(t *testing.T, uow port.UnitOfWork, eventType string) map[string]string {
	t.Helper()
	tr := uowTracker(uow)
	require.NotNil(t, tr, "uow tracker not found")
	for _, f := range tr.OutboxFacts() {
		if f.EventType == eventType {
			var m map[string]string
			require.NoError(t, jsonparser.Unmarshal(f.Payload, &m))
			return m
		}
	}
	t.Fatalf("fact with event type %s not found in outbox", eventType)
	return nil
}

func setUOWIdem(uow port.UnitOfWork, key, fingerprint string, response []byte) {
	if tr := uowTracker(uow); tr != nil {
		tr.SetIdemEntry(key, fingerprint, response)
	}
}

func uowCommitCount(uow port.UnitOfWork) int {
	if tr := uowTracker(uow); tr != nil {
		return tr.CommitCount()
	}
	return 0
}

func uowOutboxLen(uow port.UnitOfWork) int {
	if tr := uowTracker(uow); tr != nil {
		return tr.OutboxLen()
	}
	return 0
}

func setUOWFailAfterCommit(uow port.UnitOfWork, fail bool) {
	if tr := uowTracker(uow); tr != nil {
		tr.FailAfterCommit.Store(fail)
	}
}

type stagedTx struct {
	postings  map[valueobject.PostingID]entity.PostingData
	outbox    []port.OutboxFact
	reserved  map[string]*mockUOWIdemEntry
	completed map[string][]byte
}

func newStagedTx() *stagedTx {
	return &stagedTx{
		postings:  map[valueobject.PostingID]entity.PostingData{},
		reserved:  map[string]*mockUOWIdemEntry{},
		completed: map[string][]byte{},
	}
}

func assignPostingIDs(posting *entity.PostingData, cursorVal string, stagedCount int, trackerCount int64) {
	if posting.ID == "" {
		if cursorVal == "cursor-7" {
			posting.ID = valueobject.PostingID(fmt.Sprintf("50000000-0000-4000-8000-%012d", int64(stagedCount)+trackerCount+1))
		} else {
			posting.ID = valueobject.PostingID(fmt.Sprintf("123e4567-e89b-12d3-a456-%012x", 0x426614174000+int64(stagedCount)+trackerCount))
		}
	}
	for i := range posting.Entries {
		if posting.Entries[i].ID == "" {
			if cursorVal == "cursor-7" {
				posting.Entries[i].ID = valueobject.EntryID(fmt.Sprintf("70000000-0000-4000-8000-%012d", i+1))
			} else {
				posting.Entries[i].ID = valueobject.EntryID(fmt.Sprintf("123e4567-e89b-12d3-a456-%012x", 0x426614174000+int64(stagedCount)+int64(i)+1))
			}
		}
		if posting.Entries[i].PostingID == "" {
			posting.Entries[i].PostingID = posting.ID
		}
	}
}

func bindMockTxPostings(t *testing.T, mockTx *mockapplication.MockTx, cursorVal string, staged *stagedTx, tracker *mockUOWTracker) {
	var repo *mockdomain.MockPostingRepository
	if t != nil {
		repo = mockdomain.NewMockPostingRepository(t)
	} else {
		repo = &mockdomain.MockPostingRepository{}
	}
	repo.EXPECT().Commit(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, posting entity.PostingData) (entity.PostingData, error) {
		assignPostingIDs(&posting, cursorVal, len(staged.postings), tracker.postingsCount.Load())
		if _, dup := tracker.postings.Load(posting.ID); dup {
			return entity.PostingData{}, entity.NewError("POSTING_CONFLICT", "posting id already committed")
		}
		if _, dup := staged.postings[posting.ID]; dup {
			return entity.PostingData{}, entity.NewError("POSTING_CONFLICT", "posting id already committed")
		}
		staged.postings[posting.ID] = posting
		tracker.commits.Add(1)
		return posting, nil
	}).Maybe()

	repo.EXPECT().FindByID(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, id valueobject.PostingID) (entity.PostingData, error) {
		if p, ok := staged.postings[id]; ok {
			return p, nil
		}
		if val, ok := tracker.postings.Load(id); ok {
			return val.(entity.PostingData), nil
		}
		return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
	}).Maybe()

	repo.EXPECT().FindByExternalReference(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, _ valueobject.TenantID, ref string) (entity.PostingData, error) {
		for _, p := range staged.postings {
			if p.ExternalReference == ref {
				return p, nil
			}
		}
		var found *entity.PostingData
		tracker.postings.Range(func(_, val any) bool {
			p := val.(entity.PostingData)
			if p.ExternalReference == ref {
				found = &p
				return false
			}
			return true
		})
		if found != nil {
			return *found, nil
		}
		return entity.PostingData{}, entity.NewError("POSTING_NOT_FOUND", "posting is unknown")
	}).Maybe()

	repo.EXPECT().FindByAccount(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, "", nil).Maybe()

	mockTx.EXPECT().Postings().Return(repo).Maybe()
}

func bindMockTxHolds(t *testing.T, mockTx *mockapplication.MockTx) {
	var repo *mockdomain.MockHoldRepository
	if t != nil {
		repo = mockdomain.NewMockHoldRepository(t)
	} else {
		repo = &mockdomain.MockHoldRepository{}
	}
	repo.EXPECT().Create(mock.Anything, mock.Anything).Return(entity.HoldData{}, nil).Maybe()
	repo.EXPECT().FindByID(mock.Anything, mock.Anything, mock.Anything).Return(entity.HoldData{}, entity.NewError("HOLD_NOT_FOUND", "hold is unknown")).Maybe()
	repo.EXPECT().FindActiveByAccount(mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Maybe()
	repo.EXPECT().Update(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	mockTx.EXPECT().Holds().Return(repo).Maybe()
}

func reserveMockIdem(ctx context.Context, rec port.IdempotencyRecord, staged *stagedTx, tracker *mockUOWTracker) (port.ReserveOutcome, error) {
	if _, ok := staged.reserved[rec.Key]; ok {
		return port.ReserveOutcome{}, nil
	}

	entry := &mockUOWIdemEntry{
		fingerprint: rec.Fingerprint,
	}
	ch := make(chan struct{})
	entry.inFlight.Store(&ch)

	actual, loaded := tracker.idem.LoadOrStore(rec.Key, entry)
	if !loaded {
		staged.reserved[rec.Key] = entry
		return port.ReserveOutcome{}, nil
	}

	existing := actual.(*mockUOWIdemEntry)
	if existing.fingerprint != rec.Fingerprint {
		return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
	}

	for {
		if existing.completed.Load() {
			return port.ReserveOutcome{Replay: true, Response: existing.response}, nil
		}
		inFlightPtr := existing.inFlight.Load()
		if inFlightPtr == nil {
			newCh := make(chan struct{})
			if existing.inFlight.CompareAndSwap(nil, &newCh) {
				staged.reserved[rec.Key] = existing
				return port.ReserveOutcome{}, nil
			}
			continue
		}
		select {
		case <-ctx.Done():
			return port.ReserveOutcome{}, ctx.Err()
		case <-*inFlightPtr:
		}
	}
}

func bindMockTxIdempotency(t *testing.T, mockTx *mockapplication.MockTx, staged *stagedTx, tracker *mockUOWTracker) {
	var store *mockapplication.MockIdempotencyStore
	if t != nil {
		store = mockapplication.NewMockIdempotencyStore(t)
	} else {
		store = &mockapplication.MockIdempotencyStore{}
	}
	store.EXPECT().Reserve(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
		return reserveMockIdem(ctx, rec, staged, tracker)
	}).Maybe()

	store.EXPECT().Complete(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, key string, response []byte) error {
		staged.completed[key] = response
		return nil
	}).Maybe()

	mockTx.EXPECT().Idempotency().Return(store).Maybe()
}

func bindMockTxOutbox(t *testing.T, mockTx *mockapplication.MockTx, staged *stagedTx) {
	var store *mockapplication.MockEventOutbox
	if t != nil {
		store = mockapplication.NewMockEventOutbox(t)
	} else {
		store = &mockapplication.MockEventOutbox{}
	}
	appendFn := func(args mock.Arguments) {
		for i := 1; i < len(args); i++ {
			if f, ok := args.Get(i).(port.OutboxFact); ok {
				staged.outbox = append(staged.outbox, f)
			}
		}
	}
	for i := 1; i <= 5; i++ {
		matchers := make([]any, i)
		for j := range matchers {
			matchers[j] = mock.Anything
		}
		store.On("Append", matchers...).Run(appendFn).Return(nil).Maybe()
	}
	mockTx.EXPECT().Outbox().Return(store).Maybe()
}

func rollbackMockTx(staged *stagedTx, tracker *mockUOWTracker) {
	for key, entry := range staged.reserved {
		if chPtr := entry.inFlight.Swap(nil); chPtr != nil {
			close(*chPtr)
		}
		if !entry.completed.Load() {
			tracker.idem.Delete(key)
		}
	}
}

func commitMockTx(staged *stagedTx, tracker *mockUOWTracker) {
	for id, posting := range staged.postings {
		if _, loaded := tracker.postings.LoadOrStore(id, posting); !loaded {
			tracker.postingsCount.Add(1)
		}
	}
	if len(staged.outbox) > 0 {
		for {
			cur := tracker.outbox.Load()
			var next []port.OutboxFact
			if cur != nil {
				next = make([]port.OutboxFact, len(*cur)+len(staged.outbox))
				copy(next, *cur)
				copy(next[len(*cur):], staged.outbox)
			} else {
				next = append([]port.OutboxFact(nil), staged.outbox...)
			}
			if tracker.outbox.CompareAndSwap(cur, &next) {
				break
			}
		}
	}
	for key, resp := range staged.completed {
		if v, ok := tracker.idem.Load(key); ok {
			entry := v.(*mockUOWIdemEntry)
			entry.response = resp
			entry.completed.Store(true)
		}
	}
	for _, entry := range staged.reserved {
		if chPtr := entry.inFlight.Swap(nil); chPtr != nil {
			close(*chPtr)
		}
	}
}

func newMockUOW(t *testing.T, defaultCursor ...string) *mockapplication.MockUnitOfWork {
	var uow *mockapplication.MockUnitOfWork
	if t != nil {
		t.Helper()
		uow = mockapplication.NewMockUnitOfWork(t)
	} else {
		uow = &mockapplication.MockUnitOfWork{}
	}
	cursorVal := "cursor-5"
	if len(defaultCursor) > 0 && defaultCursor[0] != "" {
		cursorVal = defaultCursor[0]
	}
	tracker := &mockUOWTracker{}
	uowTrackerMap.Store(uow, tracker)

	uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, port.Tx) error) error {
		staged := newStagedTx()

		var mockTx *mockapplication.MockTx
		if t != nil {
			mockTx = mockapplication.NewMockTx(t)
		} else {
			mockTx = &mockapplication.MockTx{}
		}
		mockTx.EXPECT().Cursor().Return(cursorVal).Maybe()

		bindMockTxPostings(t, mockTx, cursorVal, staged, tracker)
		bindMockTxHolds(t, mockTx)
		bindMockTxIdempotency(t, mockTx, staged, tracker)
		bindMockTxOutbox(t, mockTx, staged)

		if err := fn(ctx, mockTx); err != nil {
			rollbackMockTx(staged, tracker)
			return err
		}

		commitMockTx(staged, tracker)

		if tracker.FailAfterCommit.Load() {
			return entity.NewError("COMMIT_AMBIGUOUS", "commit outcome unknown")
		}
		return nil
	}).Maybe()

	return uow
}
