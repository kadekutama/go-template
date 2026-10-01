package query_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

const (
	qTenant   = valueobject.TenantID("t-1")
	qLedger   = valueobject.LedgerID("l-1")
	qAccount  = valueobject.AccountID("a-1")
	qCurrency = valueobject.AssetCode("USD")
	qEuro     = valueobject.AssetCode("EUR")
)

var qAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type balanceEntriesPage struct {
	entries []entity.Entry
	next    string
}

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
			postings := mockdomain.NewMockPostingRepository(t)
			postings.EXPECT().
				FindByID(mock.Anything, qTenant, valueobject.PostingID("p-1")).
				Return(tc.posting, tc.err).
				Once()

			svc := query.NewPostingQueryService(query.PostingQueryServiceParams{Postings: postings})
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
		pages          map[string]balanceEntriesPage
		holds          []entity.HoldData
		query          port.BalanceQuery
		expectedResult port.BalanceView
		expectedError  error
	}

	testCases := []testCase{
		{
			name:    "liability available nets credits minus debits minus holds",
			account: qAccountData(valueobject.ClassLiability),
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			query: port.BalanceQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: qAccount,
			},
			expectedResult: port.BalanceView{},
			expectedError:  entity.NewError("BALANCE_ASSET_REQUIRED", "balance requires an asset code"),
		},
		{
			name:    "minInt64 entry negation returns BALANCE_OVERFLOW",
			account: qAccountData(valueobject.ClassAsset),
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			pages: map[string]balanceEntriesPage{
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
			accounts := mockdomain.NewMockAccountRepository(t)
			entries := mockdomain.NewMockEntryReader(t)
			holds := mockdomain.NewMockHoldRepository(t)
			clock := mockapplication.NewMockClock(t)

			clock.EXPECT().Now().Return(qAt).Maybe()
			accounts.EXPECT().
				FindByID(mock.Anything, tc.query.TenantID, tc.query.AccountID).
				Return(tc.account, tc.accountErr).
				Maybe()
			holds.EXPECT().
				FindActiveByAccount(mock.Anything, tc.query.TenantID, tc.query.AccountID).
				Return(tc.holds, nil).
				Maybe()

			for cursor, page := range tc.pages {
				entries.EXPECT().
					FindByAccount(mock.Anything, tc.query.TenantID, tc.query.AccountID, cursor, 500).
					Return(page.entries, page.next, nil).
					Maybe()
			}

			svc := query.NewBalanceService(query.BalanceServiceParams{
				Accounts: accounts,
				Entries:  entries,
				Holds:    holds,
				Clock:    clock,
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
		query          port.EntriesQuery
		programmed     []entity.Entry
		programmedNext string
		expectedResult port.EntriesPage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "execute returns page with next cursor",
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: 10,
			},
			programmed: []entity.Entry{
				{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
			},
			programmedNext: "cursor-next",
			expectedResult: port.EntriesPage{
				Entries: []entity.Entry{
					{ID: "e-1", AccountID: "a-1", AmountMinor: 100},
				},
				NextCursor: "cursor-next",
			},
			expectedError: nil,
		},
		{
			name: "non-positive limit returns validation error",
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: -5,
			},
			programmed:     nil,
			programmedNext: "",
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("INVALID_EXPORT_LIMIT", "export limit must be between 1 and 500"),
		},
		{
			name: "zero limit returns validation error",
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: 0,
			},
			programmed:     nil,
			programmedNext: "",
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("INVALID_EXPORT_LIMIT", "export limit must be between 1 and 500"),
		},
		{
			name: "excessive limit returns validation error",
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger, AccountID: "a-1",
				Limit: 501,
			},
			programmed:     nil,
			programmedNext: "",
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("INVALID_EXPORT_LIMIT", "export limit must be between 1 and 500"),
		},
		{
			name: "missing tenant returns validation error",
			query: port.EntriesQuery{
				TenantID: "", LedgerID: qLedger, AccountID: "a-1",
				Limit: 10,
			},
			programmed:     nil,
			programmedNext: "",
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing account returns validation error",
			query: port.EntriesQuery{
				TenantID: qTenant, LedgerID: qLedger,
				Limit: 10,
			},
			programmed:     nil,
			programmedNext: "",
			expectedResult: port.EntriesPage{},
			expectedError:  entity.NewError("ENTRIES_ACCOUNT_REQUIRED", "entries require an account id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entries := mockdomain.NewMockEntryReader(t)
			if tc.expectedError == nil {
				entries.EXPECT().
					FindByAccount(mock.Anything, tc.query.TenantID, tc.query.AccountID, tc.query.Cursor, tc.query.Limit).
					Return(tc.programmed, tc.programmedNext, nil).
					Once()
			}

			svc := query.NewEntriesService(query.EntriesServiceParams{Entries: entries})
			actualResult, err := svc.Execute(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
