package specification_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func mustMoney(minor int64, asset valueobject.AssetCode) valueobject.Money {
	return valueobject.MustMoney(minor, asset)
}

func TestTransferAmountPositive(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		candidate             specification.TransferCandidate
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "positive amount passes",
			ctx:  context.Background(),
			candidate: specification.TransferCandidate{
				From:   testAccount1,
				To:     testAccount2,
				Amount: mustMoney(50, testUSD),
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "zero amount rejected",
			ctx:  context.Background(),
			candidate: specification.TransferCandidate{
				From:   testAccount1,
				To:     testAccount2,
				Amount: mustMoney(0, testUSD),
			},
			expectedPassed:        false,
			expectedViolationCode: "INVALID_TRANSFER_AMOUNT",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.TransferAmountPositive().Evaluate(tc.ctx, tc.candidate)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestSufficientFunds(t *testing.T) {
	t.Parallel()

	spec := specification.SufficientFunds(specification.BalanceSnapshot{
		Available: mustMoney(100, testUSD),
	})

	type testCase struct {
		name                  string
		ctx                   context.Context
		amount                valueobject.Money
		expectedPassed        bool
		expectedViolationCode string
		expectedDetails       map[string]string
	}

	testCases := []testCase{
		{
			name:                  "exact available amount passes",
			ctx:                   context.Background(),
			amount:                mustMoney(100, testUSD),
			expectedPassed:        true,
			expectedViolationCode: "",
			expectedDetails:       nil,
		},
		{
			name:                  "amount exceeds available funds rejected",
			ctx:                   context.Background(),
			amount:                mustMoney(101, testUSD),
			expectedPassed:        false,
			expectedViolationCode: "INSUFFICIENT_FUNDS",
			expectedDetails: map[string]string{
				"available": "100",
				"required":  "101",
			},
		},
		{
			name:                  "cross-asset currency mismatch rejected",
			ctx:                   context.Background(),
			amount:                mustMoney(50, testEUR),
			expectedPassed:        false,
			expectedViolationCode: "CURRENCY_MISMATCH",
			expectedDetails:       nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := spec.Evaluate(tc.ctx, tc.amount)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
				if tc.expectedDetails != nil {
					for k, v := range tc.expectedDetails {
						assert.Equal(t, v, res.Violations[0].Details[k])
					}
				}
			}
		})
	}
}

func TestCaptureAmountValid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		req                   specification.CaptureRequest
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "exact capture amount passes",
			ctx:  context.Background(),
			req: specification.CaptureRequest{
				AuthorizedMinor:    100,
				CapturedTotalMinor: 60,
				CaptureMinor:       40,
				Asset:              testUSD,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "over-capture rejected",
			ctx:  context.Background(),
			req: specification.CaptureRequest{
				AuthorizedMinor:    100,
				CapturedTotalMinor: 60,
				CaptureMinor:       41,
				Asset:              testUSD,
			},
			expectedPassed:        false,
			expectedViolationCode: "CAPTURE_EXCEEDS_AUTHORIZED",
		},
		{
			name: "negative capture rejected closed",
			ctx:  context.Background(),
			req: specification.CaptureRequest{
				AuthorizedMinor:    100,
				CapturedTotalMinor: 0,
				CaptureMinor:       -1,
				Asset:              testUSD,
			},
			expectedPassed:        false,
			expectedViolationCode: "CAPTURE_EXCEEDS_AUTHORIZED",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.CaptureAmountValid().Evaluate(tc.ctx, tc.req)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestAllocationExact(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		allocation            specification.Allocation
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "exact shares pass",
			ctx:  context.Background(),
			allocation: specification.Allocation{
				Source: 100,
				Shares: []int64{34, 33, 33},
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "off-by-one under-allocated rejected",
			ctx:  context.Background(),
			allocation: specification.Allocation{
				Source: 100,
				Shares: []int64{34, 33, 32},
			},
			expectedPassed:        false,
			expectedViolationCode: "UNBALANCED_TRANSACTION",
		},
		{
			name: "empty shares rejected",
			ctx:  context.Background(),
			allocation: specification.Allocation{
				Source: 0,
				Shares: nil,
			},
			expectedPassed:        false,
			expectedViolationCode: "UNBALANCED_TRANSACTION",
		},
		{
			name: "overflowing sum rejected without wrapping",
			ctx:  context.Background(),
			allocation: specification.Allocation{
				Source: 1,
				Shares: []int64{math.MaxInt64, math.MaxInt64},
			},
			expectedPassed:        false,
			expectedViolationCode: "UNBALANCED_TRANSACTION",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.AllocationExact().Evaluate(tc.ctx, tc.allocation)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}
