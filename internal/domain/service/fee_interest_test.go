package service_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestAssessTransactionFee(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		amountMinor    int64
		bps            int64
		floorMinor     int64
		capMinor       int64
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "within bounds",
			amountMinor:    10000,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(290),
			expectedError:  nil,
		},
		{
			name:           "floor",
			amountMinor:    100,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(30),
			expectedError:  nil,
		},
		{
			name:           "cap",
			amountMinor:    1000000,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(500),
			expectedError:  nil,
		},
		{
			name:           "uncapped",
			amountMinor:    10000,
			bps:            290,
			floorMinor:     0,
			capMinor:       0,
			expectedResult: int64(290),
			expectedError:  nil,
		},
		{
			name:           "free tier skips floor",
			amountMinor:    10000,
			bps:            0,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(0),
			expectedError:  nil,
		},
		{
			name:           "zero amount",
			amountMinor:    0,
			bps:            290,
			floorMinor:     30,
			capMinor:       500,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FEE_AMOUNT", Message: "fee amount must be positive"},
		},
		{
			name:           "negative bps",
			amountMinor:    100,
			bps:            -5,
			floorMinor:     0,
			capMinor:       0,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FEE_BPS", Message: "fee basis points must be non-negative"},
		},
		{
			name:           "inverted bounds",
			amountMinor:    100,
			bps:            5,
			floorMinor:     500,
			capMinor:       100,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FEE_BOUNDS", Message: "fee cap/floor bounds are invalid"},
		},
		{
			name:           "overflow",
			amountMinor:    math.MaxInt64,
			bps:            2,
			floorMinor:     0,
			capMinor:       0,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FEE_OVERFLOW", Message: "fee computation overflowed"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.AssessTransactionFee(tc.amountMinor, tc.bps, tc.floorMinor, tc.capMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSplitFee(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		totalMinor     int64
		processorMinor int64
		expectedResult service.FeeSplit
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid split",
			totalMinor:     290,
			processorMinor: 100,
			expectedResult: service.FeeSplit{TotalMinor: 290, ProcessorMinor: 100, PlatformMinor: 190},
			expectedError:  nil,
		},
		{
			name:           "processor over total",
			totalMinor:     290,
			processorMinor: 291,
			expectedResult: service.FeeSplit{},
			expectedError:  &entity.Error{Code: "FEE_SPLIT_INVALID", Message: "fee split requires 0 <= processor <= total"},
		},
		{
			name:           "negative total",
			totalMinor:     -1,
			processorMinor: 0,
			expectedResult: service.FeeSplit{},
			expectedError:  &entity.Error{Code: "FEE_SPLIT_INVALID", Message: "fee split requires 0 <= processor <= total"},
		},
		{
			name:           "negative processor",
			totalMinor:     290,
			processorMinor: -1,
			expectedResult: service.FeeSplit{},
			expectedError:  &entity.Error{Code: "FEE_SPLIT_INVALID", Message: "fee split requires 0 <= processor <= total"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.SplitFee(tc.totalMinor, tc.processorMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestAssessMonthlyFee(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		volumeMinor    int64
		tiers          []service.MonthlyTier
		expectedResult int64
		expectedError  error
	}

	tiers := []service.MonthlyTier{{UpToMinor: 100000, BPS: 300}, {UpToMinor: 0, BPS: 200}}

	testCases := []testCase{
		{
			name:           "first tier",
			volumeMinor:    50000,
			tiers:          tiers,
			expectedResult: int64(1500),
			expectedError:  nil,
		},
		{
			name:           "top tier",
			volumeMinor:    500000,
			tiers:          tiers,
			expectedResult: int64(10000),
			expectedError:  nil,
		},
		{
			name:           "tier boundary",
			volumeMinor:    100000,
			tiers:          tiers,
			expectedResult: int64(3000),
			expectedError:  nil,
		},
		{
			name:           "negative volume",
			volumeMinor:    -1,
			tiers:          tiers,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "INVALID_FEE_AMOUNT", Message: "fee volume must be non-negative"},
		},
		{
			name:           "empty schedule",
			volumeMinor:    10,
			tiers:          nil,
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FEE_SCHEDULE_REQUIRED", Message: "monthly fee schedule is required"},
		},
		{
			name:           "bad tier",
			volumeMinor:    10,
			tiers:          []service.MonthlyTier{{UpToMinor: -1, BPS: 1}},
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FEE_SCHEDULE_INVALID", Message: "fee schedule tier is invalid"},
		},
		{
			name:           "overflow",
			volumeMinor:    math.MaxInt64,
			tiers:          []service.MonthlyTier{{UpToMinor: 0, BPS: 10000}},
			expectedResult: int64(0),
			expectedError:  &entity.Error{Code: "FEE_OVERFLOW", Message: "fee computation overflowed"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.AssessMonthlyFee(tc.volumeMinor, tc.tiers)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestAccrueDaily(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		balanceMinor    int64
		annualBPS       int64
		class           valueobject.AccountClass
		expectedResult1 int64
		expectedResult2 bool
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "liability credit side",
			balanceMinor:    100000,
			annualBPS:       365,
			class:           valueobject.ClassLiability,
			expectedResult1: 10,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "asset debit side",
			balanceMinor:    100000,
			annualBPS:       365,
			class:           valueobject.ClassAsset,
			expectedResult1: 10,
			expectedResult2: true,
			expectedError:   nil,
		},
		{
			name:            "expense debit side",
			balanceMinor:    100000,
			annualBPS:       365,
			class:           valueobject.ClassExpense,
			expectedResult1: 10,
			expectedResult2: true,
			expectedError:   nil,
		},
		{
			name:            "equity credit side",
			balanceMinor:    100000,
			annualBPS:       365,
			class:           valueobject.ClassEquity,
			expectedResult1: 10,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "revenue credit side",
			balanceMinor:    100000,
			annualBPS:       365,
			class:           valueobject.ClassRevenue,
			expectedResult1: 10,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "zero balance",
			balanceMinor:    0,
			annualBPS:       365,
			class:           valueobject.ClassLiability,
			expectedResult1: 0,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "negative balance",
			balanceMinor:    -100,
			annualBPS:       365,
			class:           valueobject.ClassLiability,
			expectedResult1: 0,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "zero rate",
			balanceMinor:    100000,
			annualBPS:       0,
			class:           valueobject.ClassLiability,
			expectedResult1: 0,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "daily divisor",
			balanceMinor:    36500,
			annualBPS:       10000,
			class:           valueobject.ClassLiability,
			expectedResult1: 100,
			expectedResult2: false,
			expectedError:   nil,
		},
		{
			name:            "bad class",
			balanceMinor:    1000,
			annualBPS:       100,
			class:           "NOPE",
			expectedResult1: 0,
			expectedResult2: false,
			expectedError:   &entity.Error{Code: "ACCOUNT_CLASS_INVALID", Message: "account class is invalid"},
		},
		{
			name:            "overflow",
			balanceMinor:    math.MaxInt64,
			annualBPS:       10000,
			class:           valueobject.ClassAsset,
			expectedResult1: 0,
			expectedResult2: false,
			expectedError:   &entity.Error{Code: "INTEREST_OVERFLOW", Message: "interest computation overflowed"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult1, actualResult2, err := service.AccrueDaily(tc.balanceMinor, tc.annualBPS, tc.class)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
