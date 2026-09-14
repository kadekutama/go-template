package valueobject_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

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
			name:           "empty base",
			base:           "",
			quote:          "USD",
			expectedResult: valueobject.FxPair{},
			expectedError:  errors.New("fx: pair requires base and quote codes"),
		},
		{
			name:           "empty quote",
			base:           "EUR",
			quote:          "",
			expectedResult: valueobject.FxPair{},
			expectedError:  errors.New("fx: pair requires base and quote codes"),
		},
		{
			name:           "identical assets",
			base:           "USD",
			quote:          "USD",
			expectedResult: valueobject.FxPair{},
			expectedError:  errors.New("fx: pair base and quote must differ"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := valueobject.NewFxPair(tc.base, tc.quote)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestFxRateValidate(t *testing.T) {
	t.Parallel()

	pair, err := valueobject.NewFxPair("EUR", "USD")
	assert.NoError(t, err)
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	baseRate := valueobject.FxRate{
		ID:          "EURUSD-1",
		Pair:        pair,
		Numerator:   10850,
		Denominator: 10000,
		Source:      "ecb",
		QuotedAt:    now,
		TTL:         time.Hour,
	}

	type testCase struct {
		name          string
		rate          valueobject.FxRate
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid rate",
			rate:          baseRate,
			expectedError: nil,
		},
		{
			name: "missing id",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.ID = ""
				return r
			}(),
			expectedError: errors.New("fx: FX_RATE_ID_REQUIRED: rate id is required"),
		},
		{
			name: "missing pair",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Pair = valueobject.FxPair{}
				return r
			}(),
			expectedError: errors.New("fx: FX_PAIR_REQUIRED: rate requires a base/quote pair"),
		},
		{
			name: "zero numerator",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Numerator = 0
				return r
			}(),
			expectedError: errors.New("fx: FX_RATE_INVALID: numerator and denominator must be positive"),
		},
		{
			name: "negative numerator",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Numerator = -10
				return r
			}(),
			expectedError: errors.New("fx: FX_RATE_INVALID: numerator and denominator must be positive"),
		},
		{
			name: "zero denominator",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Denominator = 0
				return r
			}(),
			expectedError: errors.New("fx: FX_RATE_INVALID: numerator and denominator must be positive"),
		},
		{
			name: "negative denominator",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Denominator = -10
				return r
			}(),
			expectedError: errors.New("fx: FX_RATE_INVALID: numerator and denominator must be positive"),
		},
		{
			name: "missing source",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.Source = ""
				return r
			}(),
			expectedError: errors.New("fx: FX_SOURCE_REQUIRED: rate source is required"),
		},
		{
			name: "missing quoted at",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.QuotedAt = time.Time{}
				return r
			}(),
			expectedError: errors.New("fx: FX_QUOTED_AT_REQUIRED: quote time is required"),
		},
		{
			name: "zero ttl",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.TTL = 0
				return r
			}(),
			expectedError: errors.New("fx: FX_TTL_REQUIRED: rate TTL must be positive"),
		},
		{
			name: "negative ttl",
			rate: func() valueobject.FxRate {
				r := baseRate
				r.TTL = -time.Minute
				return r
			}(),
			expectedError: errors.New("fx: FX_TTL_REQUIRED: rate TTL must be positive"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.rate.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestFxRateIsStale(t *testing.T) {
	t.Parallel()

	pair, _ := valueobject.NewFxPair("EUR", "USD")
	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	rate := valueobject.FxRate{
		ID:          "EURUSD-1",
		Pair:        pair,
		Numerator:   10850,
		Denominator: 10000,
		Source:      "ecb",
		QuotedAt:    now,
		TTL:         time.Hour,
	}

	type testCase struct {
		name           string
		rate           valueobject.FxRate
		at             time.Time
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "same time as quote",
			rate:           rate,
			at:             now,
			expectedResult: false,
		},
		{
			name:           "inside ttl window",
			rate:           rate,
			at:             now.Add(30 * time.Minute),
			expectedResult: false,
		},
		{
			name:           "at ttl expiration boundary",
			rate:           rate,
			at:             now.Add(time.Hour),
			expectedResult: false,
		},
		{
			name:           "after ttl expiration",
			rate:           rate,
			at:             now.Add(time.Hour + time.Second),
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := tc.rate.IsStale(tc.at)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
