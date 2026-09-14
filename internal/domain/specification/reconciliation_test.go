package specification_test

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestAmountWithinTolerance(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ledgerSum      int64
		externalSum    int64
		toleranceMinor int64
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "exact match",
			ledgerSum:      10000,
			externalSum:    10000,
			toleranceMinor: 0,
			expectedResult: true,
		},
		{
			name:           "within tolerance",
			ledgerSum:      10000,
			externalSum:    9999,
			toleranceMinor: 1,
			expectedResult: true,
		},
		{
			name:           "beyond tolerance",
			ledgerSum:      10000,
			externalSum:    9998,
			toleranceMinor: 1,
			expectedResult: false,
		},
		{
			name:           "negative direction within",
			ledgerSum:      9999,
			externalSum:    10000,
			toleranceMinor: 1,
			expectedResult: true,
		},
		{
			name:           "negative tolerance rejected",
			ledgerSum:      10000,
			externalSum:    10000,
			toleranceMinor: -1,
			expectedResult: false,
		},
		{
			name:           "overflow difference rejected",
			ledgerSum:      math.MaxInt64,
			externalSum:    -10,
			toleranceMinor: 100,
			expectedResult: false,
		},
		{
			name:           "opposite overflow difference rejected",
			ledgerSum:      math.MinInt64,
			externalSum:    10,
			toleranceMinor: 100,
			expectedResult: false,
		},
		{
			name:           "extreme difference min int64 rejected",
			ledgerSum:      0,
			externalSum:    math.MinInt64,
			toleranceMinor: 100,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := specification.AmountWithinTolerance(tc.ledgerSum, tc.externalSum, tc.toleranceMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestTimingWithinWindow(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		skew           time.Duration
		window         time.Duration
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "inside window",
			skew:           2 * time.Hour,
			window:         24 * time.Hour,
			expectedResult: true,
		},
		{
			name:           "negative skew inside",
			skew:           -2 * time.Hour,
			window:         24 * time.Hour,
			expectedResult: true,
		},
		{
			name:           "beyond window",
			skew:           48 * time.Hour,
			window:         24 * time.Hour,
			expectedResult: false,
		},
		{
			name:           "negative window never",
			skew:           0,
			window:         -time.Hour,
			expectedResult: false,
		},
		{
			name:           "exact boundary window",
			skew:           24 * time.Hour,
			window:         24 * time.Hour,
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := specification.TimingWithinWindow(tc.skew, tc.window)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestCanAutoResolveTiming(t *testing.T) {
	t.Parallel()

	allowedList := []string{"DATE_MISMATCH", "MISSING_IN_BANK", "MISSING_IN_LEDGER"}

	type testCase struct {
		name           string
		breakType      string
		allowed        []string
		skew           time.Duration
		maxSkew        time.Duration
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "allowed type within max skew",
			breakType:      "DATE_MISMATCH",
			allowed:        allowedList,
			skew:           2 * time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: true,
		},
		{
			name:           "amount mismatch forbidden unconditionally",
			breakType:      "AMOUNT_MISMATCH",
			allowed:        []string{"AMOUNT_MISMATCH", "DATE_MISMATCH"},
			skew:           time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: false,
		},
		{
			name:           "duplicate forbidden unconditionally",
			breakType:      "DUPLICATE",
			allowed:        []string{"DUPLICATE", "DATE_MISMATCH"},
			skew:           time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: false,
		},
		{
			name:           "type not in allowed list",
			breakType:      "MISSING_IN_LEDGER",
			allowed:        []string{"DATE_MISMATCH"},
			skew:           time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: false,
		},
		{
			name:           "skew exceeds max skew",
			breakType:      "DATE_MISMATCH",
			allowed:        allowedList,
			skew:           48 * time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: false,
		},
		{
			name:           "negative skew within max skew",
			breakType:      "DATE_MISMATCH",
			allowed:        allowedList,
			skew:           -12 * time.Hour,
			maxSkew:        24 * time.Hour,
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := specification.CanAutoResolveTiming(tc.breakType, tc.allowed, tc.skew, tc.maxSkew)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
