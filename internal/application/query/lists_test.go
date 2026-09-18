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
)

func TestListDelegates(t *testing.T) {
	t.Parallel()

	t.Run("intent list delegates", func(t *testing.T) {
		payments := new(mockapplication.MockPaymentQueryUseCases)
		payments.On("ListIntents", mock.Anything, mock.Anything, mock.Anything).Return([]port.PaymentIntentResult{
			{IntentID: "pi-1", Status: "CONFIRMED"},
		}, nil).Once()
		svc := query.NewPaymentQueryService(query.PaymentQueryServiceParams{Payments: payments})
		listed, err := svc.ListIntents(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
		payments.AssertExpectations(t)
	})

	t.Run("refund list delegates", func(t *testing.T) {
		payments := new(mockapplication.MockPaymentQueryUseCases)
		payments.On("ListRefunds", mock.Anything, mock.Anything, mock.Anything).Return([]port.RefundResult{
			{RefundID: "re-1", Status: "SUCCEEDED"},
		}, nil).Once()
		svc := query.NewRefundQueryService(query.RefundQueryServiceParams{Payments: payments})
		listed, err := svc.ListRefunds(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
		payments.AssertExpectations(t)
	})

	t.Run("payout and topup lists delegate", func(t *testing.T) {
		payments := new(mockapplication.MockPaymentQueryUseCases)
		payments.On("ListPayouts", mock.Anything, mock.Anything, mock.Anything).Return([]port.PayoutResult{
			{PayoutID: "po-1", Status: "PENDING"},
		}, nil).Once()
		payments.On("ListTopups", mock.Anything, mock.Anything, mock.Anything).Return([]port.TopupResult{
			{TopupID: "tp-1", Status: "PENDING"},
		}, nil).Once()
		payouts := query.NewPayoutQueryService(query.PayoutQueryServiceParams{Payments: payments})
		listed, err := payouts.ListPayouts(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
		topped, err := payouts.ListTopups(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, topped, 1)
		payments.AssertExpectations(t)
	})

	t.Run("transfer list reads through store", func(t *testing.T) {
		transfers := &stubTransfers{
			transfers: []command.TransferRecord{{ID: "x-1", Status: "COMPLETED"}},
		}
		svc := query.NewTransferQueryService(query.TransferQueryServiceParams{Transfers: transfers})
		page, err := svc.ListTransfers(context.Background(), port.TransferFilter{TenantID: "t-1"})
		require.NoError(t, err)
		assert.Len(t, page.Transfers, 1)
		assert.Equal(t, "x-1", page.Transfers[0].TransferID)
	})

	t.Run("transaction list delegates to posting search", func(t *testing.T) {
		search := &listPostingSearch{page: port.PostingSearchPage{
			Postings:   []entity.PostingData{{ID: "p-1"}},
			NextCursor: "",
		}}
		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{Posting: &listPostingReader{}, Query: search})
		page, err := svc.ListTransactions(context.Background(), port.PostingFilter{TenantID: "t-1"})
		require.NoError(t, err)
		assert.Len(t, page.Postings, 1)
	})
}

type listPostingReader struct{}

func (*listPostingReader) Execute(_ context.Context, _ port.GetPostingQuery) (port.PostingView, error) {
	return port.PostingView{}, nil
}

type listPostingSearch struct {
	page port.PostingSearchPage
}

func (s *listPostingSearch) Search(_ context.Context, _ port.PostingFilter) (port.PostingSearchPage, error) {
	return s.page, nil
}

func TestStatementRead(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

	t.Run("statement gathers dated lines", func(t *testing.T) {
		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
			Posting: &listPostingReader{},
			Query: &listPostingSearch{page: port.PostingSearchPage{
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
			}},
			Accounts: &listAccountsReader{account: entity.AccountData{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-1", Number: "1", Name: "a",
				Class: valueobject.ClassAsset, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1,
			}},
		})
		statement, err := svc.GetStatement(context.Background(), "t-1", "l-1", "a-1", at.Add(-time.Hour), at.Add(time.Hour))
		require.NoError(t, err)
		require.Len(t, statement.Entries, 1)
		assert.Equal(t, "e-1", string(statement.Entries[0].ID))
	})

	t.Run("cross-ledger statement rejected", func(t *testing.T) {
		svc := query.NewTransactionQueryService(query.TransactionQueryServiceParams{
			Posting: &listPostingReader{},
			Query:   &listPostingSearch{},
			Accounts: &listAccountsReader{account: entity.AccountData{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-9", Number: "1", Name: "a",
				Class: valueobject.ClassAsset, AssetCode: "USD", Status: valueobject.StatusActive, Version: 1,
			}},
		})
		_, err := svc.GetStatement(context.Background(), "t-1", "l-1", "a-1", at.Add(-time.Hour), at.Add(time.Hour))
		assert.Equal(t, entity.NewError("LEDGER_MISMATCH", "account belongs to a different ledger"), err)
	})
}

type listAccountsReader struct {
	account entity.AccountData
}

func (r *listAccountsReader) Create(_ context.Context, _ entity.AccountData) (entity.AccountData, error) {
	return entity.AccountData{}, nil
}

func (r *listAccountsReader) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID) (entity.AccountData, error) {
	return r.account, nil
}

func (r *listAccountsReader) FindByTenant(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
	return []entity.AccountData{r.account}, "", nil
}

func (r *listAccountsReader) UpdateMetadata(_ context.Context, _ entity.AccountData, _ int64) error {
	return nil
}

func (r *listAccountsReader) UpdateStatus(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ valueobject.AccountStatus, _ int64) error {
	return nil
}

func TestOpsListReads(t *testing.T) {
	t.Parallel()

	t.Run("recon run and break lists", func(t *testing.T) {
		store := &opsListReconStore{
			runs:   []command.ReconRunRecord{{ID: "run-1"}, {ID: "run-2"}},
			breaks: []command.BreakRecord{{Break: entity.ReconciliationBreak{BreakID: "b-1"}}},
		}
		svc := query.NewReconQueryService(query.ReconQueryServiceParams{Runs: store})
		runs, err := svc.ListRuns(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, runs, 2)
		breaks, err := svc.ListBreaks(context.Background(), "t-1", "", 10)
		require.NoError(t, err)
		assert.Len(t, breaks, 1)
	})

	t.Run("period list reads through", func(t *testing.T) {
		store := &opsListPeriodStore{periods: []entity.PeriodData{{ID: "2026-09"}}}
		svc := query.NewPeriodQueryService(query.PeriodQueryServiceParams{Periods: store})
		listed, err := svc.ListPeriods(context.Background(), "t-1", "l-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})

	t.Run("report list reads through", func(t *testing.T) {
		store := &opsListReportStore{records: []command.ReportRecord{{ID: "rep-1"}}}
		svc := query.NewReportQueryService(query.ReportQueryServiceParams{Reports: store})
		listed, err := svc.ListReports(context.Background(), "t-1", 10)
		require.NoError(t, err)
		assert.Len(t, listed, 1)
	})
}

type opsListReconStore struct {
	opsReconStore
	runs   []command.ReconRunRecord
	breaks []command.BreakRecord
}

func (s *opsListReconStore) ListRuns(_ context.Context, _ valueobject.TenantID, _ int) ([]command.ReconRunRecord, error) {
	return s.runs, nil
}

func (s *opsListReconStore) ListBreaks(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]command.BreakRecord, error) {
	return s.breaks, nil
}

type opsListPeriodStore struct {
	opsPeriodStore
	periods []entity.PeriodData
}

func (s *opsListPeriodStore) ListPeriods(_ context.Context, _, _ string, _ int) ([]entity.PeriodData, error) {
	return s.periods, nil
}

type opsListReportStore struct {
	opsReportStore
	records []command.ReportRecord
}

func (s *opsListReportStore) ListReports(_ context.Context, _ valueobject.TenantID, _ int) ([]command.ReportRecord, error) {
	return s.records, nil
}
