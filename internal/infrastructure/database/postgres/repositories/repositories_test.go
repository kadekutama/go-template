package repositories

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestConstructorsRequireDB(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		constructor   func() error
		expectedError error
	}

	testCases := []testCase{
		{
			name: "ledger repository needs DB",
			constructor: func() error {
				_, err := NewLedgerRepository(LedgerRepositoryParams{DB: nil})

				return err
			},
			expectedError: ErrPostgresLedgerRepositoryNeedsDB,
		},
		{
			name: "account repository needs DB",
			constructor: func() error {
				_, err := NewAccountRepository(AccountRepositoryParams{DB: nil})

				return err
			},
			expectedError: ErrPostgresAccountRepositoryNeedsDB,
		},
		{
			name: "posting repository needs DB",
			constructor: func() error {
				_, err := NewPostingRepository(PostingRepositoryParams{DB: nil})

				return err
			},
			expectedError: ErrPostgresPostingRepositoryNeedsDB,
		},
		{
			name: "hold repository needs DB",
			constructor: func() error {
				_, err := NewHoldRepository(HoldRepositoryParams{DB: nil})

				return err
			},
			expectedError: ErrPostgresHoldRepositoryNeedsDB,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.constructor()
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}

func TestTenantRequiredFailsClosed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		operation     func() error
		expectedError error
	}

	testCases := []testCase{
		{
			name: "ledger find without tenant",
			operation: func() error {
				repo := &LedgerRepository{db: nil}
				_, err := repo.FindByID(context.Background(), "", "ldg-01")

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "account find without tenant",
			operation: func() error {
				repo := &AccountRepository{db: nil}
				_, err := repo.FindByID(context.Background(), "", "acct-01")

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "posting find without tenant",
			operation: func() error {
				repo := &PostingRepository{db: nil}
				_, err := repo.FindByID(context.Background(), "", "pst-01")

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
		},
		{
			name: "hold find without tenant",
			operation: func() error {
				repo := &HoldRepository{db: nil}
				_, err := repo.FindByID(context.Background(), "", "hld-01")

				return err
			},
			expectedError: ErrPostgresTenantIsRequired,
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

func TestCheckBalanced(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		entries       []entity.Entry
		expectedError error
	}

	testCases := []testCase{
		{
			name: "balanced single asset",
			entries: []entity.Entry{
				{AccountID: "a-1", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD"},
				{AccountID: "a-2", Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: "USD"},
			},
			expectedError: nil,
		},
		{
			name: "unbalanced amounts",
			entries: []entity.Entry{
				{AccountID: "a-1", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD"},
				{AccountID: "a-2", Side: valueobject.DirectionCredit, AmountMinor: 99, AssetCode: "USD"},
			},
			expectedError: ErrPostgresUnbalancedPostingForAssetUSD,
		},
		{
			name: "one asset unbalanced among two balanced",
			entries: []entity.Entry{
				{AccountID: "a-1", Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD"},
				{AccountID: "a-2", Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: "USD"},
				{AccountID: "a-3", Side: valueobject.DirectionDebit, AmountMinor: 50, AssetCode: "EUR"},
				{AccountID: "a-4", Side: valueobject.DirectionCredit, AmountMinor: 40, AssetCode: "EUR"},
			},
			expectedError: ErrPostgresUnbalancedPostingForAssetEUR,
		},
		{
			name: "overflow fails instead of wrapping",
			entries: []entity.Entry{
				{AccountID: "a-1", Side: valueobject.DirectionDebit, AmountMinor: math.MaxInt64, AssetCode: "USD"},
				{AccountID: "a-2", Side: valueobject.DirectionDebit, AmountMinor: 1, AssetCode: "USD"},
				{AccountID: "a-3", Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: "USD"},
			},
			expectedError: ErrPostgresBalanceOverflowForAssetUSD,
		},
		{
			name:          "empty set balances trivially",
			entries:       nil,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkBalanced(tc.entries)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
