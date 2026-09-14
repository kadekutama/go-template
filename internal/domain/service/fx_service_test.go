package service_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestConvert(t *testing.T) {
	t.Parallel()

	pair, _ := valueobject.NewFxPair("EUR", "USD")
	fresh := valueobject.FxRate{
		ID:          string(pair.Base) + string(pair.Quote) + "-1",
		Pair:        pair,
		Numerator:   10850,
		Denominator: 10000,
		Source:      "test",
		QuotedAt:    time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
		TTL:         time.Hour,
	}

	type testCase struct {
		name           string
		amountMinor    int64
		rate           valueobject.FxRate
		at             time.Time
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "fresh rate",
			amountMinor:    10000,
			rate:           fresh,
			at:             fresh.QuotedAt.Add(time.Minute),
			expectedResult: int64(10850),
			expectedError:  nil,
		},
		{
			name:           "zero amount",
			amountMinor:    0,
			rate:           fresh,
			at:             fresh.QuotedAt,
			expectedResult: int64(0),
			expectedError:  nil,
		},
		{
			name:           "stale rate",
			amountMinor:    100,
			rate:           fresh,
			at:             fresh.QuotedAt.Add(2 * time.Hour),
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FX_RATE_STALE", Message: "fx rate is stale"},
		},
		{
			name:           "negative amount",
			amountMinor:    -5,
			rate:           fresh,
			at:             fresh.QuotedAt,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FX_AMOUNT", Message: "fx amount must be non-negative"},
		},
		{
			name:        "broken shape",
			amountMinor: 100,
			rate: func() valueobject.FxRate {
				r := fresh
				r.Numerator = 0
				return r
			}(),
			at:             fresh.QuotedAt,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FX_RATE_INVALID", Message: "fx rate is invalid: fx: FX_RATE_INVALID: numerator and denominator must be positive"},
		},
		{
			name:        "mul overflow",
			amountMinor: 2,
			rate: func() valueobject.FxRate {
				r := fresh
				r.Numerator = math.MaxInt64
				return r
			}(),
			at:             fresh.QuotedAt,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FX_OVERFLOW", Message: "fx conversion overflowed"},
		},
		{
			name:        "add overflow",
			amountMinor: math.MaxInt64,
			rate: func() valueobject.FxRate {
				r := fresh
				r.Numerator, r.Denominator = 1, 3
				return r
			}(),
			at:             fresh.QuotedAt,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FX_OVERFLOW", Message: "fx conversion overflowed"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.Convert(tc.amountMinor, tc.rate, tc.at)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestGainLoss(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name                string
		authorizeQuoteMinor int64
		settleQuoteMinor    int64
		expectedResult      int64
	}
	testCases := []testCase{
		{
			name:                "gain",
			authorizeQuoteMinor: 10850,
			settleQuoteMinor:    10900,
			expectedResult:      int64(50),
		},
		{
			name:                "loss",
			authorizeQuoteMinor: 10900,
			settleQuoteMinor:    10850,
			expectedResult:      int64(-50),
		},
		{
			name:                "flat",
			authorizeQuoteMinor: 10850,
			settleQuoteMinor:    10850,
			expectedResult:      int64(0),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.GainLoss(tc.authorizeQuoteMinor, tc.settleQuoteMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestLotsBalance(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name          string
		lots          []service.LotTotals
		expectedError error
	}
	testCases := []testCase{
		{
			name: "independent lots balance",
			lots: []service.LotTotals{
				{Asset: "EUR", Debits: 10000, Credits: 10000},
				{Asset: "USD", Debits: 10850, Credits: 10850},
			},
			expectedError: nil,
		},
		{
			name: "unbalanced lot",
			lots: []service.LotTotals{
				{Asset: "EUR", Debits: 10000, Credits: 9000},
			},
			expectedError: &entity.Error{Code: "UNBALANCED_TRANSACTION", Message: "fx lot must balance independently"},
		},
		{
			name: "missing asset",
			lots: []service.LotTotals{
				{Debits: 1, Credits: 1},
			},
			expectedError: &entity.Error{Code: "FX_LOT_ASSET_REQUIRED", Message: "fx lot requires an asset code"},
		},
		{
			name: "non-positive legs",
			lots: []service.LotTotals{
				{Asset: "USD", Debits: 0, Credits: 1},
			},
			expectedError: &entity.Error{Code: "INVALID_ENTRY_AMOUNT", Message: "fx lot legs must be positive"},
		},
		{
			name:          "empty lots",
			lots:          []service.LotTotals(nil),
			expectedError: &entity.Error{Code: "FX_LOTS_REQUIRED", Message: "fx conversion requires at least one currency lot"},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.LotsBalance(tc.lots)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestNewFxPair(t *testing.T) {
	t.Parallel()
	type testCase struct {
		name           string
		base           valueobject.AssetCode
		quote          valueobject.AssetCode
		expectedResult valueobject.FxPair
		expectedError  error
	}
	testCases := []testCase{
		{
			name:           "valid pair",
			base:           "EUR",
			quote:          "USD",
			expectedResult: valueobject.FxPair{Base: "EUR", Quote: "USD"},
			expectedError:  nil,
		},
		{
			name:           "same code",
			base:           "USD",
			quote:          "USD",
			expectedResult: valueobject.FxPair{},
			expectedError:  errors.New("fx: pair base and quote must differ"),
		},
		{
			name:           "empty code",
			base:           "",
			quote:          "USD",
			expectedResult: valueobject.FxPair{},
			expectedError:  errors.New("fx: pair requires base and quote codes"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.NewFxPair(tc.base, tc.quote)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
