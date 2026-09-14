package event_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/event"
)

func catalogMeta() event.EventMetadata {
	return event.EventMetadata{TenantID: testTenantID, LedgerID: testLedgerID, CorrelationID: testCorrID}
}

func catalogAt() time.Time { return time.Date(2026, 9, 14, 4, 0, 0, 0, time.UTC) }

func TestCatalogEventTypes(t *testing.T) {
	t.Parallel()
	at, meta := catalogAt(), catalogMeta()
	entries := []event.EntryPayload{
		{EntryID: "e-1", AccountID: testAccount1, AccountNumber: "1000", Direction: "DEBIT", AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
		{EntryID: "e-2", AccountID: testAccount2, AccountNumber: "2000", Direction: "CREDIT", AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
	}
	type testCase struct {
		expectedType string
		make         func() (event.DomainEvent, error)
	}
	testCases := []testCase{
		{"account.created.v1", func() (event.DomainEvent, error) {
			return event.NewAccountCreated(testEvent1, testAccount1, at, 1, 0, event.AccountCreatedPayload{AccountID: testAccount1, TenantID: testTenantID, AccountNumber: "1000", Name: "Cash", Type: "ASSET", AssetCode: testUSD, Status: "ACTIVE", OpenedBy: testUserID}, meta)
		}},
		{"account.updated.v1", func() (event.DomainEvent, error) {
			return event.NewAccountUpdated(testEvent1, testAccount1, at, 2, 1, event.AccountUpdatedPayload{AccountID: testAccount1, TenantID: testTenantID, Name: "Cash+", UpdatedBy: testUserID}, meta)
		}},
		{"account.frozen.v1", func() (event.DomainEvent, error) {
			return event.NewAccountFrozen(testEvent1, testAccount1, at, 3, 2, event.AccountFrozenPayload{AccountID: testAccount1, TenantID: testTenantID, Reason: "review", FrozenBy: testUserID}, meta)
		}},
		{"account.unfrozen.v1", func() (event.DomainEvent, error) {
			return event.NewAccountUnfrozen(testEvent1, testAccount1, at, 4, 3, event.AccountUnfrozenPayload{AccountID: testAccount1, TenantID: testTenantID, UnfrozenBy: testUserID}, meta)
		}},
		{"account.closed.v1", func() (event.DomainEvent, error) {
			return event.NewAccountClosed(testEvent1, testAccount1, at, 5, 4, event.AccountClosedPayload{AccountID: testAccount1, TenantID: testTenantID, Reason: "obsolete", ClosedBy: testUserID}, meta)
		}},
		{"account.verified.v1", func() (event.DomainEvent, error) {
			return event.NewAccountVerified(testEvent1, testAccount1, at, 6, 5, event.AccountVerifiedPayload{AccountID: testAccount1, TenantID: testTenantID, VerifiedBy: testUserID}, meta)
		}},
		{"account.balance.changed.v1", func() (event.DomainEvent, error) {
			return event.NewBalanceChanged(testEvent1, testAccount1, at, 7, 6, event.BalanceChangedPayload{AccountID: testAccount1, TenantID: testTenantID, AssetCode: testUSD, PostedMinor: 100, AvailableMinor: 100, LedgerCursor: 42, AsOf: at, ReferenceType: "posting", ReferenceID: testPosting1}, meta)
		}},
		{"tenant.created.v1", func() (event.DomainEvent, error) {
			return event.NewTenantCreated(testEvent1, "tn-1", at, 1, 0, event.TenantCreatedPayload{TenantID: testTenantID, LedgerID: testLedgerID, Name: "Acme", Region: "us", BaseAssetCode: testUSD, CreatedAt: at}, meta)
		}},
		{"pii.erased.v1", func() (event.DomainEvent, error) {
			return event.NewPIIErased(testEvent1, "s-1", at, 1, 0, event.PIIErasedPayload{ErasureID: "er-1", TenantID: testTenantID, SubjectType: "user", SubjectID: "u-9", PolicyVersion: "v3", ErasedAt: at}, meta)
		}},
		{"transaction.posted.v1", func() (event.DomainEvent, error) {
			return event.NewTransactionPosted(testEvent1, testPosting1, at, 1, 0, event.TransactionPostedPayload{PostingID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID, Operation: testTransferOp, Entries: entries, RecordedAt: at}, meta)
		}},
		{"transaction.reversed.v1", func() (event.DomainEvent, error) {
			return event.NewTransactionReversed(testEvent1, testPosting2, at, 1, 0, event.TransactionReversedPayload{PostingID: testPosting2, TenantID: testTenantID, OriginalPostingID: testPosting1, ReversalType: "FULL", ReversedAmountMinor: 100, ReversedEntries: entries, Reason: "error", ReversedAt: at}, meta)
		}},
		{"transaction.failed.v1", func() (event.DomainEvent, error) {
			return event.NewTransactionFailed(testEvent1, "att-1", at, 1, 0, event.TransactionFailedPayload{AttemptID: "att-1", TenantID: testTenantID, Code: "UNBALANCED_TRANSACTION", Message: "unbalanced"}, meta)
		}},
		{"transaction.pending.v1", func() (event.DomainEvent, error) {
			return event.NewTransactionPending(testEvent1, "att-1", at, 1, 0, event.TransactionPendingPayload{AttemptID: "att-1", TenantID: testTenantID, Operation: testTransferOp}, meta)
		}},
		{"transfer.created.v1", func() (event.DomainEvent, error) {
			return event.NewTransferCreated(testEvent1, testTransfer1, at, 1, 0, event.TransferCreatedPayload{TransferID: testTransfer1, TenantID: testTenantID, SourceAccount: testAccount1, DestAccount: testAccount2, AmountMinor: 50, AssetCode: testUSD}, meta)
		}},
		{"transfer.completed.v1", func() (event.DomainEvent, error) {
			return event.NewTransferCompleted(testEvent1, testTransfer1, at, 1, 0, event.TransferCompletedPayload{TransferID: testTransfer1, TenantID: testTenantID, PostingID: testPosting1}, meta)
		}},
		{"transfer.failed.v1", func() (event.DomainEvent, error) {
			return event.NewTransferFailed(testEvent1, testTransfer1, at, 1, 0, event.TransferFailedPayload{TransferID: testTransfer1, TenantID: testTenantID, Code: "INSUFFICIENT_FUNDS", Message: "low"}, meta)
		}},
		{"transfer.canceled.v1", func() (event.DomainEvent, error) {
			return event.NewTransferCanceled(testEvent1, testTransfer1, at, 1, 0, event.TransferCanceledPayload{TransferID: testTransfer1, TenantID: testTenantID, CancelledBy: testUserID}, meta)
		}},
		{"transfer.batch.received.v1", func() (event.DomainEvent, error) {
			return event.NewTransferBatchReceived(testEvent1, testBatch1, at, 1, 0, event.TransferBatchReceivedPayload{BatchID: testBatch1, TenantID: testTenantID, ItemCount: 10}, meta)
		}},
		{"transfer.batch.completed.v1", func() (event.DomainEvent, error) {
			return event.NewTransferBatchCompleted(testEvent1, testBatch1, at, 1, 0, event.TransferBatchCompletedPayload{BatchID: testBatch1, TenantID: testTenantID, CompletedCount: 9, FailedCount: 1}, meta)
		}},
		{"payment_intent.created.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentIntentCreated(testEvent1, testPayment1, at, 1, 0, event.PaymentIntentCreatedPayload{PaymentID: testPayment1, TenantID: testTenantID, AmountMinor: 100, AssetCode: testUSD}, meta)
		}},
		{"payment_intent.succeeded.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentIntentSucceeded(testEvent1, testPayment1, at, 1, 0, event.PaymentSucceededPayload{PaymentID: testPayment1, TenantID: testTenantID, AmountMinor: 100, AssetCode: testUSD}, meta)
		}},
		{"payment_intent.failed.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentIntentFailed(testEvent1, testPayment1, at, 1, 0, event.PaymentFailedPayload{PaymentID: testPayment1, TenantID: testTenantID, Code: "DECLINED", Message: "no"}, meta)
		}},
		{"payment_intent.canceled.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentIntentCanceled(testEvent1, testPayment1, at, 1, 0, event.PaymentCanceledPayload{PaymentID: testPayment1, TenantID: testTenantID, CancelledBy: testUserID}, meta)
		}},
		{"payment_intent.requires_action.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentIntentRequiresAction(testEvent1, testPayment1, at, 1, 0, event.PaymentActionRequiredPayload{PaymentID: testPayment1, TenantID: testTenantID, ActionType: "3ds", ActionURL: "https://x"}, meta)
		}},
		{"payment.captured.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentCaptured(testEvent1, testPayment1, at, 1, 0, event.PaymentCapturedPayload{PaymentID: testPayment1, TenantID: testTenantID, PostingID: testPosting1, CapturedMinor: 100, AssetCode: testUSD}, meta)
		}},
		{"payment.settled.v1", func() (event.DomainEvent, error) {
			return event.NewPaymentSettled(testEvent1, testPayment1, at, 1, 0, event.PaymentSettledPayload{PaymentID: testPayment1, TenantID: testTenantID, Provider: "stripe", PostingID: testPosting1, AmountMinor: 100, AssetCode: testUSD, SettledAt: at}, meta)
		}},
		{"refund.created.v1", func() (event.DomainEvent, error) {
			return event.NewRefundCreated(testEvent1, testRefund1, at, 1, 0, event.RefundCreatedPayload{RefundID: testRefund1, TenantID: testTenantID, OriginalPostingID: testPosting1, RefundAmountMinor: 40, AssetCode: testUSD}, meta)
		}},
		{"refund.succeeded.v1", func() (event.DomainEvent, error) {
			return event.NewRefundSucceeded(testEvent1, testRefund1, at, 1, 0, event.RefundSucceededPayload{RefundID: testRefund1, TenantID: testTenantID, PostingID: testPosting2}, meta)
		}},
		{"refund.failed.v1", func() (event.DomainEvent, error) {
			return event.NewRefundFailed(testEvent1, testRefund1, at, 1, 0, event.RefundFailedPayload{RefundID: testRefund1, TenantID: testTenantID, Code: "WINDOW", Message: "late"}, meta)
		}},
		{"payout.created.v1", func() (event.DomainEvent, error) {
			return event.NewPayoutCreated(testEvent1, testPayout1, at, 1, 0, event.PayoutCreatedPayload{PayoutID: testPayout1, TenantID: testTenantID, AccountID: testAccount1, AmountMinor: 100, AssetCode: testUSD, Method: "ach"}, meta)
		}},
		{"payout.pending.v1", func() (event.DomainEvent, error) {
			return event.NewPayoutPending(testEvent1, testPayout1, at, 1, 0, event.PayoutPendingPayload{PayoutID: testPayout1, TenantID: testTenantID, ProviderID: "pr-1"}, meta)
		}},
		{"payout.paid.v1", func() (event.DomainEvent, error) {
			return event.NewPayoutPaid(testEvent1, testPayout1, at, 1, 0, event.PayoutPaidPayload{PayoutID: testPayout1, TenantID: testTenantID, PostingID: "p-9"}, meta)
		}},
		{"payout.failed.v1", func() (event.DomainEvent, error) {
			return event.NewPayoutFailed(testEvent1, testPayout1, at, 1, 0, event.PayoutFailedPayload{PayoutID: testPayout1, TenantID: testTenantID, Code: "REJECT", Message: "no"}, meta)
		}},
		{"reconciliation.run.started.v1", func() (event.DomainEvent, error) {
			return event.NewReconciliationRunStarted(testEvent1, testRun1, at, 1, 0, event.ReconciliationRunStartedPayload{RunID: testRun1, TenantID: testTenantID, LedgerID: testLedgerID, StartedAt: at}, meta)
		}},
		{"reconciliation.run.completed.v1", func() (event.DomainEvent, error) {
			return event.NewReconciliationRunCompleted(testEvent1, testRun1, at, 1, 0, event.ReconciliationRunCompletedPayload{RunID: testRun1, TenantID: testTenantID, MatchedCount: 5, BreakCount: 1}, meta)
		}},
		{"reconciliation.break.found.v1", func() (event.DomainEvent, error) {
			return event.NewReconciliationBreakFound(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakFoundPayload{BreakID: testBreak1, TenantID: testTenantID, RunID: testRun1, BreakType: "amount"}, meta)
		}},
		{"reconciliation.break.resolved.v1", func() (event.DomainEvent, error) {
			return event.NewReconciliationBreakResolved(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakResolvedPayload{BreakID: testBreak1, TenantID: testTenantID, ResolvedBy: testUserID, Resolution: "adjust"}, meta)
		}},
		{"reconciliation.break.acknowledged.v1", func() (event.DomainEvent, error) {
			return event.NewReconciliationBreakAcknowledged(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakAcknowledgedPayload{BreakID: testBreak1, TenantID: testTenantID, AcknowledgedBy: testUserID}, meta)
		}},
		{"period.opened.v1", func() (event.DomainEvent, error) {
			return event.NewPeriodOpened(testEvent1, testPeriod1, at, 1, 0, event.PeriodOpenedPayload{PeriodID: testPeriod1, TenantID: testTenantID, LedgerID: testLedgerID, Start: at, Timezone: "UTC"}, meta)
		}},
		{"period.closed.v1", func() (event.DomainEvent, error) {
			return event.NewPeriodClosed(testEvent1, testPeriod1, at, 1, 0, event.PeriodClosedPayload{PeriodID: testPeriod1, TenantID: testTenantID, ClosedBy: testUserID}, meta)
		}},
		{"period.reopened.v1", func() (event.DomainEvent, error) {
			return event.NewPeriodReopened(testEvent1, testPeriod1, at, 1, 0, event.PeriodReopenedPayload{PeriodID: testPeriod1, TenantID: testTenantID, ReopenedBy: "admin", Reason: "fix"}, meta)
		}},
		{"fx.rate.updated.v1", func() (event.DomainEvent, error) {
			return event.NewFxRateUpdated(testEvent1, "fx-1", at, 1, 0, event.FxRateUpdatedPayload{RateID: "fx-1", TenantID: testTenantID, FromAsset: "EUR", ToAsset: testUSD, Rate: "1.0850", Source: "ecb", QuotedAt: at}, meta)
		}},
		{"fee.assessed.v1", func() (event.DomainEvent, error) {
			return event.NewFeeAssessed(testEvent1, testFee1, at, 1, 0, event.FeeAssessedPayload{FeeID: testFee1, TenantID: testTenantID, AccountID: testAccount1, AmountMinor: 290, AssetCode: testUSD, FeeType: "processing"}, meta)
		}},
		{"fee.collected.v1", func() (event.DomainEvent, error) {
			return event.NewFeeCollected(testEvent1, testFee1, at, 1, 0, event.FeeCollectedPayload{FeeID: testFee1, TenantID: testTenantID, PostingID: "p-3"}, meta)
		}},
		{"report.generated.v1", func() (event.DomainEvent, error) {
			return event.NewReportGenerated(testEvent1, "rep-1", at, 1, 0, event.ReportGeneratedPayload{ReportID: "rep-1", TenantID: testTenantID, Template: "trial_balance", Format: "csv", PeriodStart: "2026-09-01", PeriodEnd: "2026-09-30"}, meta)
		}},
		{"dispute.opened.v1", func() (event.DomainEvent, error) {
			return event.NewDisputeOpened(testEvent1, testDispute1, at, 1, 0, event.DisputeOpenedPayload{DisputeID: testDispute1, TenantID: testTenantID, TransactionID: testPosting1, Network: "VISA", AmountMinor: 100, FeeMinor: 1500, AssetCode: testUSD, EvidenceDueAt: "2026-10-01T00:00:00Z"}, meta)
		}},
		{"dispute.closed.v1", func() (event.DomainEvent, error) {
			return event.NewDisputeClosed(testEvent1, testDispute1, at, 1, 0, event.DisputeClosedPayload{DisputeID: testDispute1, TenantID: testTenantID, Outcome: "WON"}, meta)
		}},
		{"topup.succeeded.v1", func() (event.DomainEvent, error) {
			return event.NewTopUpSucceeded(testEvent1, testTopUp1, at, 1, 0, event.TopUpPayload{TopUpID: testTopUp1, TenantID: testTenantID, AccountID: testAccount1, AmountMinor: 500, AssetCode: testUSD, BankAccountID: "ba-1"}, meta)
		}},
		{"topup.failed.v1", func() (event.DomainEvent, error) {
			return event.NewTopUpFailed(testEvent1, testTopUp1, at, 1, 0, event.TopUpFailedPayload{TopUpID: testTopUp1, TenantID: testTenantID, Code: "REJECT", Message: "no"}, meta)
		}},
	}
	if len(testCases) != 49 {
		t.Fatalf("catalog table has %d entries, want 49", len(testCases))
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.expectedType, func(t *testing.T) {
			t.Parallel()
			evt, err := tc.make()
			if err != nil {
				t.Fatalf("constructor: %v", err)
			}
			if evt.EventType() != tc.expectedType {
				t.Errorf("EventType = %q, want %q", evt.EventType(), tc.expectedType)
			}
			if evt.EventID() != testEvent1 || evt.Metadata().TenantID != testTenantID {
				t.Errorf("envelope broken: %+v", evt)
			}
			if !evt.OccurredAt().Equal(at) {
				t.Errorf("OccurredAt = %v", evt.OccurredAt())
			}
		})
	}
}

func TestCatalogValidation(t *testing.T) {
	t.Parallel()

	at, meta := catalogAt(), catalogMeta()

	type testCase struct {
		name          string
		call          func() error
		expectedError bool
	}

	testCases := []testCase{
		{
			name: "empty account payload",
			call: func() error {
				_, err := event.NewAccountCreated(testEvent1, testAccount1, at, 1, 0, event.AccountCreatedPayload{}, meta)
				return err
			},
			expectedError: true,
		},
		{
			name: "zero transfer amount",
			call: func() error {
				_, err := event.NewTransferCreated(testEvent1, testTransfer1, at, 1, 0, event.TransferCreatedPayload{TransferID: testTransfer1, TenantID: testTenantID, AmountMinor: 0}, meta)
				return err
			},
			expectedError: true,
		},
		{
			name: "bad dispute outcome",
			call: func() error {
				_, err := event.NewDisputeClosed(testEvent1, testDispute1, at, 1, 0, event.DisputeClosedPayload{DisputeID: testDispute1, TenantID: testTenantID, Outcome: "MAYBE"}, meta)
				return err
			},
			expectedError: true,
		},
		{
			name: "posted transaction without entries",
			call: func() error {
				_, err := event.NewTransactionPosted(testEvent1, testPosting1, at, 1, 0, event.TransactionPostedPayload{PostingID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID}, meta)
				return err
			},
			expectedError: true,
		},
		{
			name: "empty tenant metadata",
			call: func() error {
				badMeta := event.EventMetadata{}
				_, err := event.NewPayoutCreated(testEvent1, testPayout1, at, 1, 0, event.PayoutCreatedPayload{PayoutID: testPayout1, TenantID: testTenantID, AccountID: testAccount1, AmountMinor: 1}, badMeta)
				return err
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.call()
			if tc.expectedError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestTransactionPostedPreservesEntries(t *testing.T) {
	t.Parallel()
	at, meta := catalogAt(), catalogMeta()
	entries := []event.EntryPayload{
		{EntryID: "e-1", AccountID: testAccount1, Direction: "DEBIT", AmountMinor: 100, AssetCode: testUSD, AccountSeq: 3},
		{EntryID: "e-2", AccountID: testAccount2, Direction: "CREDIT", AmountMinor: 100, AssetCode: testUSD, AccountSeq: 9},
	}
	evt, err := event.NewTransactionPosted(testEvent1, testPosting1, at, 1, 0, event.TransactionPostedPayload{PostingID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID, Operation: testTransferOp, Entries: entries, RecordedAt: at}, meta)
	if err != nil {
		t.Fatalf("constructor: %v", err)
	}
	got := evt.Typed().Entries
	if len(got) != 2 || got[0].AccountSeq != 3 || got[1].AccountSeq != 9 {
		t.Fatalf("entries not preserved: %+v", got)
	}
	entries[0].AmountMinor = 999
	if evt.Typed().Entries[0].AmountMinor != 100 {
		t.Fatal("posted entries must be defensively copied")
	}
}

func TestPIIErasedCarriesNoValue(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(event.PIIErasedPayload{})
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "value" || name == "erased_value" || name == "pii" {
			t.Errorf("PIIErasedPayload must not expose erased values (field %s)", name)
		}
	}
}

func TestCatalogValidationMatrix(t *testing.T) {
	t.Parallel()
	at, meta := catalogAt(), catalogMeta()
	bad := event.EventMetadata{}
	type testCase struct {
		name string
		call func() error
	}
	testCases := []testCase{
		{"account.created empty", func() error {
			_, err := event.NewAccountCreated(testEvent1, testAccount1, at, 1, 0, event.AccountCreatedPayload{}, meta)
			return err
		}},
		{"account.updated empty", func() error {
			_, err := event.NewAccountUpdated(testEvent1, testAccount1, at, 1, 0, event.AccountUpdatedPayload{}, meta)
			return err
		}},
		{"account.frozen empty", func() error {
			_, err := event.NewAccountFrozen(testEvent1, testAccount1, at, 1, 0, event.AccountFrozenPayload{}, meta)
			return err
		}},
		{"account.frozen no reason", func() error {
			_, err := event.NewAccountFrozen(testEvent1, testAccount1, at, 1, 0, event.AccountFrozenPayload{AccountID: testAccount1, TenantID: testTenantID}, meta)
			return err
		}},
		{"account.unfrozen empty", func() error {
			_, err := event.NewAccountUnfrozen(testEvent1, testAccount1, at, 1, 0, event.AccountUnfrozenPayload{}, meta)
			return err
		}},
		{"account.closed empty", func() error {
			_, err := event.NewAccountClosed(testEvent1, testAccount1, at, 1, 0, event.AccountClosedPayload{}, meta)
			return err
		}},
		{"account.closed no reason", func() error {
			_, err := event.NewAccountClosed(testEvent1, testAccount1, at, 1, 0, event.AccountClosedPayload{AccountID: testAccount1, TenantID: testTenantID}, meta)
			return err
		}},
		{"account.verified empty", func() error {
			_, err := event.NewAccountVerified(testEvent1, testAccount1, at, 1, 0, event.AccountVerifiedPayload{}, meta)
			return err
		}},
		{"balance.changed empty", func() error {
			_, err := event.NewBalanceChanged(testEvent1, testAccount1, at, 1, 0, event.BalanceChangedPayload{}, meta)
			return err
		}},
		{"tenant.created empty", func() error {
			_, err := event.NewTenantCreated(testEvent1, testTenant1, at, 1, 0, event.TenantCreatedPayload{}, meta)
			return err
		}},
		{"pii.erased empty", func() error {
			_, err := event.NewPIIErased(testEvent1, testSubject1, at, 1, 0, event.PIIErasedPayload{}, meta)
			return err
		}},
		{"transaction.posted empty", func() error {
			_, err := event.NewTransactionPosted(testEvent1, testPosting1, at, 1, 0, event.TransactionPostedPayload{}, meta)
			return err
		}},
		{"transaction.reversed empty", func() error {
			_, err := event.NewTransactionReversed(testEvent1, testPosting1, at, 1, 0, event.TransactionReversedPayload{}, meta)
			return err
		}},
		{"transaction.reversed no reason", func() error {
			_, err := event.NewTransactionReversed(testEvent1, testPosting1, at, 1, 0, event.TransactionReversedPayload{PostingID: testPosting1, TenantID: testTenantID, OriginalPostingID: testOrigPosting}, meta)
			return err
		}},
		{"transaction.failed empty", func() error {
			_, err := event.NewTransactionFailed(testEvent1, testAccount1, at, 1, 0, event.TransactionFailedPayload{}, meta)
			return err
		}},
		{"transaction.pending empty", func() error {
			_, err := event.NewTransactionPending(testEvent1, testAccount1, at, 1, 0, event.TransactionPendingPayload{}, meta)
			return err
		}},
		{"transfer.created empty", func() error {
			_, err := event.NewTransferCreated(testEvent1, testTransfer1, at, 1, 0, event.TransferCreatedPayload{}, meta)
			return err
		}},
		{"transfer.created negative", func() error {
			_, err := event.NewTransferCreated(testEvent1, testTransfer1, at, 1, 0, event.TransferCreatedPayload{TransferID: testTransfer1, TenantID: testTenantID, AmountMinor: -1}, meta)
			return err
		}},
		{"transfer.completed empty", func() error {
			_, err := event.NewTransferCompleted(testEvent1, testTransfer1, at, 1, 0, event.TransferCompletedPayload{}, meta)
			return err
		}},
		{"transfer.failed empty", func() error {
			_, err := event.NewTransferFailed(testEvent1, testTransfer1, at, 1, 0, event.TransferFailedPayload{}, meta)
			return err
		}},
		{"transfer.canceled empty", func() error {
			_, err := event.NewTransferCanceled(testEvent1, testTransfer1, at, 1, 0, event.TransferCanceledPayload{}, meta)
			return err
		}},
		{"batch.received empty", func() error {
			_, err := event.NewTransferBatchReceived(testEvent1, testBatch1, at, 1, 0, event.TransferBatchReceivedPayload{}, meta)
			return err
		}},
		{"batch.received zero items", func() error {
			_, err := event.NewTransferBatchReceived(testEvent1, testBatch1, at, 1, 0, event.TransferBatchReceivedPayload{BatchID: testBatch1, TenantID: testTenantID}, meta)
			return err
		}},
		{"batch.completed empty", func() error {
			_, err := event.NewTransferBatchCompleted(testEvent1, testBatch1, at, 1, 0, event.TransferBatchCompletedPayload{}, meta)
			return err
		}},
		{"intent.created empty", func() error {
			_, err := event.NewPaymentIntentCreated(testEvent1, testPayment1, at, 1, 0, event.PaymentIntentCreatedPayload{}, meta)
			return err
		}},
		{"intent.succeeded empty", func() error {
			_, err := event.NewPaymentIntentSucceeded(testEvent1, testPayment1, at, 1, 0, event.PaymentSucceededPayload{}, meta)
			return err
		}},
		{"intent.failed empty", func() error {
			_, err := event.NewPaymentIntentFailed(testEvent1, testPayment1, at, 1, 0, event.PaymentFailedPayload{}, meta)
			return err
		}},
		{"intent.canceled empty", func() error {
			_, err := event.NewPaymentIntentCanceled(testEvent1, testPayment1, at, 1, 0, event.PaymentCanceledPayload{}, meta)
			return err
		}},
		{"requires_action empty", func() error {
			_, err := event.NewPaymentIntentRequiresAction(testEvent1, testPayment1, at, 1, 0, event.PaymentActionRequiredPayload{}, meta)
			return err
		}},
		{"captured empty", func() error {
			_, err := event.NewPaymentCaptured(testEvent1, testPayment1, at, 1, 0, event.PaymentCapturedPayload{}, meta)
			return err
		}},
		{"captured zero", func() error {
			_, err := event.NewPaymentCaptured(testEvent1, testPayment1, at, 1, 0, event.PaymentCapturedPayload{PaymentID: testPayment1, TenantID: testTenantID, PostingID: testPosting1}, meta)
			return err
		}},
		{"settled empty", func() error {
			_, err := event.NewPaymentSettled(testEvent1, testPayment1, at, 1, 0, event.PaymentSettledPayload{}, meta)
			return err
		}},
		{"refund.created empty", func() error {
			_, err := event.NewRefundCreated(testEvent1, testRefund1, at, 1, 0, event.RefundCreatedPayload{}, meta)
			return err
		}},
		{"refund.created zero", func() error {
			_, err := event.NewRefundCreated(testEvent1, testRefund1, at, 1, 0, event.RefundCreatedPayload{RefundID: testRefund1, TenantID: testTenantID, OriginalPostingID: testPosting1}, meta)
			return err
		}},
		{"refund.succeeded empty", func() error {
			_, err := event.NewRefundSucceeded(testEvent1, testRefund1, at, 1, 0, event.RefundSucceededPayload{}, meta)
			return err
		}},
		{"refund.failed empty", func() error {
			_, err := event.NewRefundFailed(testEvent1, testRefund1, at, 1, 0, event.RefundFailedPayload{}, meta)
			return err
		}},
		{"payout.created empty", func() error {
			_, err := event.NewPayoutCreated(testEvent1, testPayout1, at, 1, 0, event.PayoutCreatedPayload{}, meta)
			return err
		}},
		{"payout.created zero", func() error {
			_, err := event.NewPayoutCreated(testEvent1, testPayout1, at, 1, 0, event.PayoutCreatedPayload{PayoutID: testPayout1, TenantID: testTenantID, AccountID: testAccount1}, meta)
			return err
		}},
		{"payout.pending empty", func() error {
			_, err := event.NewPayoutPending(testEvent1, testPayout1, at, 1, 0, event.PayoutPendingPayload{}, meta)
			return err
		}},
		{"payout.paid empty", func() error {
			_, err := event.NewPayoutPaid(testEvent1, testPayout1, at, 1, 0, event.PayoutPaidPayload{}, meta)
			return err
		}},
		{"payout.failed empty", func() error {
			_, err := event.NewPayoutFailed(testEvent1, testPayout1, at, 1, 0, event.PayoutFailedPayload{}, meta)
			return err
		}},
		{"run.started empty", func() error {
			_, err := event.NewReconciliationRunStarted(testEvent1, testRun1, at, 1, 0, event.ReconciliationRunStartedPayload{}, meta)
			return err
		}},
		{"run.completed empty", func() error {
			_, err := event.NewReconciliationRunCompleted(testEvent1, testRun1, at, 1, 0, event.ReconciliationRunCompletedPayload{}, meta)
			return err
		}},
		{"break.found empty", func() error {
			_, err := event.NewReconciliationBreakFound(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakFoundPayload{}, meta)
			return err
		}},
		{"break.resolved empty", func() error {
			_, err := event.NewReconciliationBreakResolved(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakResolvedPayload{}, meta)
			return err
		}},
		{"break.acked empty", func() error {
			_, err := event.NewReconciliationBreakAcknowledged(testEvent1, testBreak1, at, 1, 0, event.ReconciliationBreakAcknowledgedPayload{}, meta)
			return err
		}},
		{"period.opened empty", func() error {
			_, err := event.NewPeriodOpened(testEvent1, testPeriod1, at, 1, 0, event.PeriodOpenedPayload{}, meta)
			return err
		}},
		{"period.closed empty", func() error {
			_, err := event.NewPeriodClosed(testEvent1, testPeriod1, at, 1, 0, event.PeriodClosedPayload{}, meta)
			return err
		}},
		{"period.reopened empty", func() error {
			_, err := event.NewPeriodReopened(testEvent1, testPeriod1, at, 1, 0, event.PeriodReopenedPayload{}, meta)
			return err
		}},
		{"fx empty", func() error {
			_, err := event.NewFxRateUpdated(testEvent1, testFx1, at, 1, 0, event.FxRateUpdatedPayload{}, meta)
			return err
		}},
		{"fx no rate", func() error {
			_, err := event.NewFxRateUpdated(testEvent1, testFx1, at, 1, 0, event.FxRateUpdatedPayload{RateID: testFx1, TenantID: testTenantID, FromAsset: "EUR", ToAsset: testUSD}, meta)
			return err
		}},
		{"fee.assessed empty", func() error {
			_, err := event.NewFeeAssessed(testEvent1, testFee1, at, 1, 0, event.FeeAssessedPayload{}, meta)
			return err
		}},
		{"fee.assessed zero", func() error {
			_, err := event.NewFeeAssessed(testEvent1, testFee1, at, 1, 0, event.FeeAssessedPayload{FeeID: testFee1, TenantID: testTenantID, AccountID: testAccount1}, meta)
			return err
		}},
		{"fee.collected empty", func() error {
			_, err := event.NewFeeCollected(testEvent1, testFee1, at, 1, 0, event.FeeCollectedPayload{}, meta)
			return err
		}},
		{"report empty", func() error {
			_, err := event.NewReportGenerated(testEvent1, testReport1, at, 1, 0, event.ReportGeneratedPayload{}, meta)
			return err
		}},
		{"dispute.opened empty", func() error {
			_, err := event.NewDisputeOpened(testEvent1, testDispute1, at, 1, 0, event.DisputeOpenedPayload{}, meta)
			return err
		}},
		{"dispute.opened zero", func() error {
			_, err := event.NewDisputeOpened(testEvent1, testDispute1, at, 1, 0, event.DisputeOpenedPayload{DisputeID: testDispute1, TenantID: testTenantID, TransactionID: testPosting1, Network: "VISA"}, meta)
			return err
		}},
		{"dispute.closed empty", func() error {
			_, err := event.NewDisputeClosed(testEvent1, testDispute1, at, 1, 0, event.DisputeClosedPayload{}, meta)
			return err
		}},
		{"dispute.closed bad outcome", func() error {
			_, err := event.NewDisputeClosed(testEvent1, testDispute1, at, 1, 0, event.DisputeClosedPayload{DisputeID: testDispute1, TenantID: testTenantID, Outcome: "DRAW"}, meta)
			return err
		}},
		{"topup.succeeded empty", func() error {
			_, err := event.NewTopUpSucceeded(testEvent1, testTopUp1, at, 1, 0, event.TopUpPayload{}, meta)
			return err
		}},
		{"topup.succeeded zero", func() error {
			_, err := event.NewTopUpSucceeded(testEvent1, testTopUp1, at, 1, 0, event.TopUpPayload{TopUpID: testTopUp1, TenantID: testTenantID, AccountID: testAccount1}, meta)
			return err
		}},
		{"topup.failed empty", func() error {
			_, err := event.NewTopUpFailed(testEvent1, testTopUp1, at, 1, 0, event.TopUpFailedPayload{}, meta)
			return err
		}},
		{"empty envelope ids", func() error {
			_, err := event.NewPayoutPending("", testPayout1, at, 1, 0, event.PayoutPendingPayload{PayoutID: testPayout1, TenantID: testTenantID}, meta)
			return err
		}},
		{"empty tenant metadata", func() error {
			_, err := event.NewPayoutPending(testEvent1, testPayout1, at, 1, 0, event.PayoutPendingPayload{PayoutID: testPayout1, TenantID: testTenantID}, bad)
			return err
		}},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.call(); err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}
