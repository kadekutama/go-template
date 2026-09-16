package query_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	qTenant   = valueobject.TenantID("t-1")
	qLedger   = valueobject.LedgerID("l-1")
	qAccount  = valueobject.AccountID("a-1")
	qCurrency = valueobject.AssetCode("USD")
	qEuro     = valueobject.AssetCode("EUR")
)

var qAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type stubPostings struct {
	posting entity.PostingData
	err     error
}

func (s *stubPostings) Commit(_ context.Context, _ entity.PostingData) error {
	return nil
}

func (s *stubPostings) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.PostingID) (entity.PostingData, error) {
	return s.posting, s.err
}

func (s *stubPostings) FindByExternalReference(_ context.Context, _ valueobject.TenantID, _ string) (entity.PostingData, error) {
	return entity.PostingData{}, s.err
}

func (s *stubPostings) FindByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ string, _ int) ([]entity.PostingData, string, error) {
	return nil, "", s.err
}

type entriesPage struct {
	entries []entity.Entry
	next    string
}

type stubEntries struct {
	pages          map[string]entriesPage
	receivedLimits []int
	err            error
}

func (s *stubEntries) FindByPosting(_ context.Context, _ valueobject.TenantID, _ valueobject.PostingID) ([]entity.Entry, error) {
	return nil, s.err
}

func (s *stubEntries) FindByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, cursor string, limit int) ([]entity.Entry, string, error) {
	s.receivedLimits = append(s.receivedLimits, limit)
	if s.err != nil {
		return nil, "", s.err
	}
	page := s.pages[cursor]
	return page.entries, page.next, nil
}

type stubAccounts struct {
	account  entity.AccountData
	accounts []entity.AccountData
	next     string
	err      error
}

func (s *stubAccounts) Create(_ context.Context, _ entity.AccountData) error {
	return nil
}

func (s *stubAccounts) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID) (entity.AccountData, error) {
	return s.account, s.err
}

func (s *stubAccounts) FindByTenant(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return s.accounts, s.next, nil
}

func (s *stubAccounts) UpdateMetadata(_ context.Context, _ entity.AccountData, _ int64) error {
	return nil
}

func (s *stubAccounts) UpdateStatus(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID, _ valueobject.AccountStatus, _ int64) error {
	return nil
}

type stubTenants struct {
	tenant  entity.TenantData
	tenants []entity.TenantData
	names   []string
	err     error
}

func (s *stubTenants) Create(_ context.Context, _ entity.TenantData) error { return nil }
func (s *stubTenants) FindByID(_ context.Context, _ valueobject.TenantID) (entity.TenantData, error) {
	return s.tenant, s.err
}
func (s *stubTenants) ListNames(_ context.Context) ([]string, error) {
	return s.names, s.err
}
func (s *stubTenants) UpdateSettings(_ context.Context, _ entity.TenantData, _ int64) error {
	return nil
}
func (s *stubTenants) ListTenants(_ context.Context) ([]entity.TenantData, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.tenants, nil
}

type stubTransfers struct {
	transfer   port.TransferRecord
	transfers  []port.TransferRecord
	batch      port.BatchRecord
	batchItems []port.BatchItem
	next       string
	err        error
}

func (s *stubTransfers) CreateTransfer(_ context.Context, _ port.TransferRecord) error { return nil }
func (s *stubTransfers) FindTransfer(_ context.Context, _ valueobject.TenantID, _ string) (port.TransferRecord, error) {
	return s.transfer, s.err
}
func (s *stubTransfers) UpdateTransfer(_ context.Context, _ port.TransferRecord) error { return nil }
func (s *stubTransfers) ListTransfers(_ context.Context, _ port.TransferListFilter) ([]port.TransferRecord, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return s.transfers, s.next, nil
}
func (s *stubTransfers) CreateBatch(_ context.Context, _ port.BatchRecord, _ []port.BatchItem) error {
	return nil
}
func (s *stubTransfers) FindBatch(_ context.Context, _ valueobject.TenantID, _ string) (port.BatchRecord, error) {
	return s.batch, s.err
}
func (s *stubTransfers) UpdateBatchState(_ context.Context, _ valueobject.TenantID, _, _ string) error {
	return nil
}
func (s *stubTransfers) UpdateBatchItem(_ context.Context, _ valueobject.TenantID, _ string, _ int, _, _ string) error {
	return nil
}
func (s *stubTransfers) ListBatchItems(_ context.Context, _ valueobject.TenantID, _ string) ([]port.BatchItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.batchItems, nil
}

type stubDisputes struct {
	dispute  entity.Dispute
	disputes []entity.Dispute
	next     string
	err      error
}

func (s *stubDisputes) CreateDispute(_ context.Context, _ entity.Dispute) error { return nil }
func (s *stubDisputes) FindDispute(_ context.Context, _ string) (entity.Dispute, error) {
	return s.dispute, s.err
}
func (s *stubDisputes) UpdateDispute(_ context.Context, _ entity.Dispute) error { return nil }
func (s *stubDisputes) ListDisputes(_ context.Context, _ string, _, _ time.Time, _ string, _ int) ([]entity.Dispute, string, error) {
	if s.err != nil {
		return nil, "", s.err
	}
	return s.disputes, s.next, nil
}

type stubHolds struct {
	holds []entity.HoldData
	err   error
}

func (s *stubHolds) Create(_ context.Context, _ entity.HoldData) error {
	return nil
}

func (s *stubHolds) FindByID(_ context.Context, _ valueobject.TenantID, _ valueobject.HoldID) (entity.HoldData, error) {
	return entity.HoldData{}, s.err
}

func (s *stubHolds) FindActiveByAccount(_ context.Context, _ valueobject.TenantID, _ valueobject.AccountID) ([]entity.HoldData, error) {
	return s.holds, s.err
}

func (s *stubHolds) Update(_ context.Context, _ entity.HoldData, _ int64) error {
	return s.err
}

type stubClock struct{}

func (stubClock) Now() time.Time { return qAt }

func qEntry(id, posting string, side valueobject.Direction, amount int64, asset valueobject.AssetCode) entity.Entry {
	return entity.Entry{
		ID:          valueobject.EntryID(id),
		PostingID:   valueobject.PostingID(posting),
		AccountID:   qAccount,
		Side:        side,
		AmountMinor: amount,
		AssetCode:   asset,
		AccountSeq:  1,
	}
}

func qAccountData(class valueobject.AccountClass) entity.AccountData {
	return entity.AccountData{
		ID: qAccount, TenantID: qTenant, LedgerID: qLedger,
		Number: "5000", Name: "q", Class: class,
		AssetCode: qCurrency, Status: valueobject.StatusActive, Version: 1,
	}
}

func TestPostingQueryExecute(t *testing.T) {
	t.Parallel()

	stored := entity.PostingData{
		ID: "p-1", TenantID: qTenant, LedgerID: qLedger, Operation: "transfer",
		Entries:     []entity.Entry{qEntry("e-1", "p-1", valueobject.DirectionDebit, 5000, qCurrency)},
		EffectiveAt: qAt, RecordedAt: qAt,
	}

	type testCase struct {
		name           string
		posting        entity.PostingData
		err            error
		expectedResult port.PostingView
		expectedError  error
	}

	testCases := []testCase{
		{
			name:    "strong read returns posting with empty cursor",
			posting: stored,
			err:     nil,
			expectedResult: port.PostingView{
				Posting: stored,
				Cursor:  "",
			},
			expectedError: nil,
		},
		{
			name:           "missing posting propagates",
			posting:        entity.PostingData{},
			err:            entity.NewError("POSTING_NOT_FOUND", "posting is unknown"),
			expectedResult: port.PostingView{},
			expectedError:  entity.NewError("POSTING_NOT_FOUND", "posting is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewPostingQueryService(query.PostingQueryServiceParams{Postings: &stubPostings{posting: tc.posting, err: tc.err}})
			actualResult, err := svc.Execute(context.Background(), port.GetPostingQuery{TenantID: qTenant, PostingID: "p-1"})
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBalanceExecute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		account        entity.AccountData
		accountErr     error
		pages          map[string]entriesPage
		holds          []entity.HoldData
		query          port.BalanceQuery
		expectedResult port.BalanceView
		expectedError  error
	}

	testCases := []testCase{
		{
			name:    "liability available nets credits minus debits minus holds",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionCredit, 10000, qCurrency),
						qEntry("e-2", "p-2", valueobject.DirectionDebit, 3000, qCurrency),
						qEntry("e-3", "p-3", valueobject.DirectionCredit, 200, qEuro),
					},
					next: "",
				},
			},
			holds: []entity.HoldData{
				{ID: "h-1", TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency, AmountMinor: 500, State: entity.HoldActive},
			},
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{
				AccountID: qAccount, AssetCode: qCurrency, AvailableMinor: 6500, AsOf: qAt,
			},
			expectedError: nil,
		},
		{
			name:    "asset available nets debits minus credits",
			account: qAccountData(valueobject.ClassAsset),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionDebit, 10000, qCurrency),
						qEntry("e-2", "p-2", valueobject.DirectionCredit, 2000, qCurrency),
					},
					next: "",
				},
			},
			holds: nil,
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{
				AccountID: qAccount, AssetCode: qCurrency, AvailableMinor: 8000, AsOf: qAt,
			},
			expectedError: nil,
		},
		{
			name:    "multipage scan follows cursors",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionCredit, 10000, qCurrency),
					},
					next: "c-1",
				},
				"c-1": {
					entries: []entity.Entry{
						qEntry("e-2", "p-2", valueobject.DirectionDebit, 1000, qCurrency),
					},
					next: "",
				},
			},
			holds: nil,
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{
				AccountID: qAccount, AssetCode: qCurrency, AvailableMinor: 9000, AsOf: qAt,
			},
			expectedError: nil,
		},
		{
			name:    "released holds do not reduce available",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionCredit, 10000, qCurrency),
					},
					next: "",
				},
			},
			holds: []entity.HoldData{
				{ID: "h-1", TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency, AmountMinor: 4000, State: entity.HoldReleased},
			},
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{
				AccountID: qAccount, AssetCode: qCurrency, AvailableMinor: 10000, AsOf: qAt,
			},
			expectedError: nil,
		},
		{
			name:       "unknown account propagates",
			account:    entity.AccountData{},
			accountErr: entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
			pages:      nil,
			holds:      nil,
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
		},
		{
			name:    "missing asset rejected",
			account: qAccountData(valueobject.ClassAsset),
			pages:   nil,
			holds:   nil,
			query: func() port.BalanceQuery {
				q := port.BalanceQuery{TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount}
				return q
			}(),
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_ASSET_REQUIRED", "balance requires an asset code"),
		},
		{
			name:    "minInt64 entry negation returns BALANCE_OVERFLOW",
			account: qAccountData(valueobject.ClassAsset),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-min", "p-min", valueobject.DirectionCredit, math.MinInt64, qCurrency),
					},
					next: "",
				},
			},
			holds: nil,
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed"),
		},
		{
			name:    "entry scan addition overflow returns BALANCE_OVERFLOW",
			account: qAccountData(valueobject.ClassAsset),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-max1", "p-max1", valueobject.DirectionDebit, math.MaxInt64-5, qCurrency),
						qEntry("e-max2", "p-max2", valueobject.DirectionDebit, 10, qCurrency),
					},
					next: "",
				},
			},
			holds: nil,
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed"),
		},
		{
			name:    "expired holds are excluded from hold summation",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionCredit, 10000, qCurrency),
					},
					next: "",
				},
			},
			holds: []entity.HoldData{
				{
					ID:          "h-expired",
					TenantID:    qTenant,
					LedgerID:    qLedger,
					AccountID:   qAccount,
					AssetCode:   qCurrency,
					AmountMinor: 4000,
					State:       entity.HoldActive,
					ExpiresAt:   qAt.Add(-time.Hour),
				},
				{
					ID:          "h-active",
					TenantID:    qTenant,
					LedgerID:    qLedger,
					AccountID:   qAccount,
					AssetCode:   qCurrency,
					AmountMinor: 1000,
					State:       entity.HoldActive,
					ExpiresAt:   qAt.Add(time.Hour),
				},
			},
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{
				AccountID:      qAccount,
				AssetCode:      qCurrency,
				AvailableMinor: 9000,
				AsOf:           qAt,
			},
			expectedError: nil,
		},
		{
			name:    "hold sum addition overflow returns BALANCE_OVERFLOW",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-1", "p-1", valueobject.DirectionCredit, 10000, qCurrency),
					},
					next: "",
				},
			},
			holds: []entity.HoldData{
				{
					ID:          "h-1",
					TenantID:    qTenant,
					LedgerID:    qLedger,
					AccountID:   qAccount,
					AssetCode:   qCurrency,
					AmountMinor: math.MaxInt64 - 5,
					State:       entity.HoldActive,
				},
				{
					ID:          "h-2",
					TenantID:    qTenant,
					LedgerID:    qLedger,
					AccountID:   qAccount,
					AssetCode:   qCurrency,
					AmountMinor: 10,
					State:       entity.HoldActive,
				},
			},
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed"),
		},
		{
			name:    "available balance subtraction overflow returns BALANCE_OVERFLOW",
			account: qAccountData(valueobject.ClassAsset),
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						qEntry("e-under1", "p-under1", valueobject.DirectionCredit, math.MaxInt64, qCurrency),
						qEntry("e-under2", "p-under2", valueobject.DirectionCredit, 1, qCurrency),
					},
					next: "",
				},
			},
			holds: []entity.HoldData{
				{
					ID:          "h-1",
					TenantID:    qTenant,
					LedgerID:    qLedger,
					AccountID:   qAccount,
					AssetCode:   qCurrency,
					AmountMinor: 1,
					State:       entity.HoldActive,
				},
			},
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount, AssetCode: qCurrency,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_OVERFLOW", "balance computation overflowed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewBalanceService(query.BalanceServiceParams{
				Accounts: &stubAccounts{account: tc.account, err: tc.accountErr},
				Entries:  &stubEntries{pages: tc.pages},
				Holds:    &stubHolds{holds: tc.holds},
				Clock:    stubClock{},
			})
			actualResult, err := svc.Execute(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestEntriesExecute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		pages          map[string]entriesPage
		query          port.EntriesQuery
		expectedResult port.EntriesPage
		expectedError  error
		expectedLimits []int
	}

	testCases := []testCase{
		{
			name: "execute returns page with next cursor",
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
					},
					next: "cursor-next",
				},
			},
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: 10,
			},
			expectedResult: port.EntriesPage{
				Entries: []entity.Entry{
					{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
				},
				NextCursor: "cursor-next",
			},
			expectedError:  nil,
			expectedLimits: []int{10},
		},
		{
			name: "clamp limits non-positive to 500 and maximum to 500",
			pages: map[string]entriesPage{
				"": {
					entries: []entity.Entry{
						{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
					},
				},
			},
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: -5,
			},
			expectedResult: port.EntriesPage{
				Entries: []entity.Entry{
					{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
				},
			},
			expectedError:  nil,
			expectedLimits: []int{500},
		},
		{
			name:  "missing tenant returns validation error",
			pages: nil,
			query: port.EntriesQuery{
				TenantID: "", LedgerID: qLedger, AccountID: "a-1",
			},
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("TENANT_REQUIRED", "tenant id is required"),
			expectedLimits: nil,
		},
		{
			name:  "missing account returns validation error",
			pages: nil,
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger,
			},
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("ENTRIES_ACCOUNT_REQUIRED", "entries require an account id"),
			expectedLimits: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entries := &stubEntries{pages: tc.pages}
			svc := query.NewEntriesService(query.EntriesServiceParams{Entries: entries})
			actualResult, err := svc.Execute(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedLimits, entries.receivedLimits)
		})
	}
}
