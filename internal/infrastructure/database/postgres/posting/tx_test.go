package posting

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestNewUnitOfWorkRequiresDB(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         UnitOfWorkParams
		expectedResult *UnitOfWork
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil DB rejected",
			params: UnitOfWorkParams{
				DB: nil,
			},
			expectedResult: nil,
			expectedError:  errors.New("postgres: unit of work needs DB"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow, err := NewUnitOfWork(tc.params)
			assert.Equal(t, tc.expectedResult, uow)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDoRequiresCallback(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		fn            func(ctx context.Context, tx appport.Tx) error
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil callback rejected before touching the database",
			ctx:           context.Background(),
			fn:            nil,
			expectedError: errors.New("postgres: unit of work callback is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &UnitOfWork{}
			err := uow.Do(tc.ctx, tc.fn)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestLockOrderDeterministic(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ids            []valueobject.AccountID
		expectedResult []valueobject.AccountID
	}

	testCases := []testCase{
		{
			name:           "empty stays empty",
			ids:            nil,
			expectedResult: []valueobject.AccountID{},
		},
		{
			name: "single unchanged",
			ids: []valueobject.AccountID{
				"acct-b",
			},
			expectedResult: []valueobject.AccountID{
				"acct-b",
			},
		},
		{
			name: "reverse sorted ascending",
			ids: []valueobject.AccountID{
				"acct-c",
				"acct-b",
				"acct-a",
			},
			expectedResult: []valueobject.AccountID{
				"acct-a",
				"acct-b",
				"acct-c",
			},
		},
		{
			name: "already sorted stable",
			ids: []valueobject.AccountID{
				"acct-a",
				"acct-b",
			},
			expectedResult: []valueobject.AccountID{
				"acct-a",
				"acct-b",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := LockOrder(tc.ids)
			assert.Equal(t, tc.expectedResult, actual)
			assert.Equal(t, tc.expectedResult, LockOrder(actual), "lock order must be idempotent")
		})
	}
}

func TestAvailableMinor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		entries        []entity.Entry
		holds          []entity.HoldData
		normalSide     []valueobject.Direction
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "debits minus credits minus active holds default debit normal",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: 100000, AssetCode: "USD"},
				{Side: valueobject.DirectionCredit, AmountMinor: 25000, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 5000, State: entity.HoldActive},
			},
			normalSide:     nil,
			expectedResult: 70000,
			expectedError:  nil,
		},
		{
			name:           "no entries no holds is zero",
			entries:        nil,
			holds:          nil,
			normalSide:     nil,
			expectedResult: 0,
			expectedError:  nil,
		},
		{
			name: "released and expired holds excluded",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: "USD"},
			},
			holds: func() []entity.HoldData {
				now := time.Now()
				return []entity.HoldData{
					{AmountMinor: 3000, State: entity.HoldReleased, ExpiresAt: now},
					{AmountMinor: 2000, State: entity.HoldExpired, ExpiresAt: now},
					{AmountMinor: 1000, State: entity.HoldCaptured, ExpiresAt: now},
				}
			}(),
			normalSide:     nil,
			expectedResult: 10000,
			expectedError:  nil,
		},
		{
			name: "entry total overflow fails",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: math.MaxInt64, AssetCode: "USD"},
				{Side: valueobject.DirectionDebit, AmountMinor: 1, AssetCode: "USD"},
			},
			holds:          nil,
			normalSide:     nil,
			expectedResult: 0,
			expectedError:  errors.New("postgres: entry total overflow"),
		},
		{
			name: "hold total overflow fails",
			entries: []entity.Entry{
				{Side: valueobject.DirectionCredit, AmountMinor: 10, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: math.MaxInt64, State: entity.HoldActive},
				{AmountMinor: 100, State: entity.HoldActive},
			},
			normalSide:     nil,
			expectedResult: 0,
			expectedError:  errors.New("postgres: hold total overflow"),
		},
		{
			name: "hold subtraction may go negative without overflow",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 200, State: entity.HoldActive},
			},
			normalSide:     nil,
			expectedResult: -100,
			expectedError:  nil,
		},
		{
			name: "credit normal side increases with credit decreases with debit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionCredit, AmountMinor: 50000, AssetCode: "USD"},
				{Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 5000, State: entity.HoldActive},
			},
			normalSide: []valueobject.Direction{
				valueobject.DirectionCredit,
			},
			expectedResult: 35000,
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := AvailableMinor(tc.entries, tc.holds, tc.normalSide...)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAvailableMinorForClass(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		entries        []entity.Entry
		holds          []entity.HoldData
		class          valueobject.AccountClass
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "asset class debit normal increases with debit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: 20000, AssetCode: "USD"},
				{Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 2000, State: entity.HoldActive},
			},
			class:          valueobject.ClassAsset,
			expectedResult: 13000,
			expectedError:  nil,
		},
		{
			name: "liability class credit normal increases with credit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionCredit, AmountMinor: 50000, AssetCode: "USD"},
				{Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 5000, State: entity.HoldActive},
			},
			class:          valueobject.ClassLiability,
			expectedResult: 35000,
			expectedError:  nil,
		},
		{
			name: "equity class credit normal increases with credit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionCredit, AmountMinor: 100000, AssetCode: "USD"},
			},
			holds:          nil,
			class:          valueobject.ClassEquity,
			expectedResult: 100000,
			expectedError:  nil,
		},
		{
			name: "revenue class credit normal increases with credit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionCredit, AmountMinor: 45000, AssetCode: "USD"},
				{Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: "USD"},
			},
			holds:          nil,
			class:          valueobject.ClassRevenue,
			expectedResult: 40000,
			expectedError:  nil,
		},
		{
			name: "expense class debit normal increases with debit",
			entries: []entity.Entry{
				{Side: valueobject.DirectionDebit, AmountMinor: 15000, AssetCode: "USD"},
				{Side: valueobject.DirectionCredit, AmountMinor: 3000, AssetCode: "USD"},
			},
			holds: []entity.HoldData{
				{AmountMinor: 1000, State: entity.HoldActive},
			},
			class:          valueobject.ClassExpense,
			expectedResult: 11000,
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := AvailableMinorForClass(tc.entries, tc.holds, tc.class)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
