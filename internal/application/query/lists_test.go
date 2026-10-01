package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

func TestListDelegates(t *testing.T) {
	t.Parallel()

	t.Run("intent list delegates", func(t *testing.T) {
		payments := mockapplication.NewMockPaymentQueryUseCases(t)
		payments.EXPECT().
			ListIntents(mock.Anything, mock.Anything, mock.Anything).
			Return([]port.PaymentIntentResult{
				{IntentID: "pi-1", Status: "CONFIRMED"},
			}, nil).
			Once()

		svc := query.NewPaymentQueryService(query.PaymentQueryServiceParams{Payments: payments})
		listed, err := svc.ListIntents(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})

	t.Run("refund list delegates", func(t *testing.T) {
		payments := mockapplication.NewMockPaymentQueryUseCases(t)
		payments.EXPECT().
			ListRefunds(mock.Anything, mock.Anything, mock.Anything).
			Return([]port.RefundResult{
				{RefundID: "re-1", Status: "SUCCEEDED"},
			}, nil).
			Once()

		svc := query.NewRefundQueryService(query.RefundQueryServiceParams{Payments: payments})
		listed, err := svc.ListRefunds(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})

	t.Run("payout and topup lists delegate", func(t *testing.T) {
		payments := mockapplication.NewMockPaymentQueryUseCases(t)
		payments.EXPECT().
			ListPayouts(mock.Anything, mock.Anything, mock.Anything).
			Return([]port.PayoutResult{
				{PayoutID: "po-1", Status: "PENDING"},
			}, nil).
			Once()
		payments.EXPECT().
			ListTopups(mock.Anything, mock.Anything, mock.Anything).
			Return([]port.TopupResult{
				{TopupID: "tp-1", Status: "PENDING"},
			}, nil).
			Once()

		payouts := query.NewPayoutQueryService(query.PayoutQueryServiceParams{Payments: payments})
		listed, err := payouts.ListPayouts(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
		topped, err := payouts.ListTopups(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, topped, 1)
	})

	t.Run("transfer list reads through store", func(t *testing.T) {
		transfers := mockcommand.NewMockTransferStore(t)
		transfers.EXPECT().
			ListTransfers(mock.Anything, mock.Anything).
			Return([]command.TransferRecord{{ID: "x-1", Status: "COMPLETED"}}, "", nil).
			Once()

		svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: transfers})
		page, err := svc.ListTransfers(context.Background(), port.TransferFilter{TenantID: "t-1"})
		require.NoError(t, err)
		assert.Len(t, page.Transfers, 1)
		assert.Equal(t, "x-1", page.Transfers[0].TransferID)
	})

	t.Run("transaction list delegates to posting search", func(t *testing.T) {
		search := mockapplication.NewMockPostingQuery(t)
		search.EXPECT().
			Search(mock.Anything, mock.Anything).
			Return(port.PostingSearchPage{
				Postings:   []entity.PostingData{{ID: "p-1"}},
				NextCursor: "",
			}, nil).
			Once()

		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
			Posting: mockapplication.NewMockGetPosting(t),
			Query:   search,
		})
		page, err := svc.ListTransactions(context.Background(), port.PostingFilter{TenantID: "t-1"})
		require.NoError(t, err)
		assert.Len(t, page.Postings, 1)
	})
}

func TestStatementRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	t.Run("statement gathers dated lines", func(t *testing.T) {
		search := mockapplication.NewMockPostingQuery(t)
		search.EXPECT().
			Search(mock.Anything, mock.Anything).
			Return(port.PostingSearchPage{
				Postings: []entity.PostingData{
					{
						ID: "p-1", TenantID: "t-1", LedgerID: "l-1", Operation: "transfer",
						Entries: []entity.Entry{
							{ID: "e-1", PostingID: "p-1", AccountID: "a-1", Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
							{ID: "e-2", PostingID: "p-1", AccountID: "a-9", Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: "USD", AccountSeq: 1},
						},
						EffectiveAt: at, RecordedAt: at,
					},
				},
			}, nil).
			Once()

		accounts := mockdomain.NewMockAccountRepository(t)
		accounts.EXPECT().
			FindByID(mock.Anything, valueobject.TenantID("t-1"), valueobject.AccountID("a-1")).
			Return(entity.AccountData{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-1", Number: "1", Name: "a",
				Class: valueobject.ClassAsset, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1,
			}, nil).
			Once()

		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
			Posting:  mockapplication.NewMockGetPosting(t),
			Query:    search,
			Accounts: accounts,
		})
		statement, err := svc.GetStatement(context.Background(), "t-1", "l-1", "a-1", at.Add(-time.Hour), at.Add(time.Hour))
		require.NoError(t, err)
		require.Len(t, statement.Entries, 1)
		assert.Equal(t, "e-1", string(statement.Entries[0].ID))
	})

	t.Run("cross-ledger statement rejected", func(t *testing.T) {
		accounts := mockdomain.NewMockAccountRepository(t)
		accounts.EXPECT().
			FindByID(mock.Anything, valueobject.TenantID("t-1"), valueobject.AccountID("a-1")).
			Return(entity.AccountData{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-9", Number: "1", Name: "a",
				Class: valueobject.ClassAsset, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1,
			}, nil).
			Once()

		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
			Posting:  mockapplication.NewMockGetPosting(t),
			Query:    mockapplication.NewMockPostingQuery(t),
			Accounts: accounts,
		})
		_, err := svc.GetStatement(context.Background(), "t-1", "l-1", "a-1", at.Add(-time.Hour), at.Add(time.Hour))
		assert.Equal(t, entity.NewError("LEDGER_MISMATCH", "account belongs to a different ledger"), err)
	})
}

func TestOpsListReads(t *testing.T) {
	t.Parallel()

	t.Run("recon run and break lists", func(t *testing.T) {
		store := mockcommand.NewMockReconStore(t)
		store.EXPECT().
			ListRuns(mock.Anything, valueobject.TenantID("t-1"), 10).
			Return([]command.ReconRunRecord{{ID: "run-1"}, {ID: "run-2"}}, nil).
			Once()
		store.EXPECT().
			ListBreaks(mock.Anything, valueobject.TenantID("t-1"), "", 10).
			Return([]command.BreakRecord{{Break: entity.ReconciliationBreak{BreakID: "b-1"}}}, nil).
			Once()

		svc := query.NewReconQueryService(query.ReconQueryServiceParams{Runs: store})
		runs, err := svc.ListRuns(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
		breaks, err := svc.ListBreaks(context.Background(), "t-1", "", 10)
		require.NoError(t, err)
		assert.Len(t, breaks, 1)
	})

	t.Run("period list reads through", func(t *testing.T) {
		store := mockcommand.NewMockPeriodStore(t)
		store.EXPECT().
			ListPeriods(mock.Anything, "t-1", "l-1", 10).
			Return([]entity.PeriodData{{ID: "2026-09"}}, nil).
			Once()

		svc := query.NewPeriodQueryService(query.PeriodQueryServiceParams{Periods: store})
		listed, err := svc.ListPeriods(context.Background(), "t-1", "l-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})

	t.Run("report list reads through", func(t *testing.T) {
		store := mockcommand.NewMockReportStore(t)
		store.EXPECT().
			ListReports(mock.Anything, valueobject.TenantID("t-1"), 10).
			Return([]command.ReportRecord{{ID: "rep-1"}}, nil).
			Once()

		svc := query.NewReportQueryService(query.ReportQueryServiceParams{Reports: store})
		listed, err := svc.ListReports(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})
}
