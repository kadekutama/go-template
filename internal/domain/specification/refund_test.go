package specification_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestRefundWindowValid(t *testing.T) {
	t.Parallel()

	posted := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name                  string
		ctx                   context.Context
		window                specification.RefundWindow
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "inside window passes",
			ctx:  context.Background(),
			window: specification.RefundWindow{
				OriginalPostedAt: posted,
				Now:              posted.Add(24 * time.Hour),
				Window:           30 * 24 * time.Hour,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "outside window rejected",
			ctx:  context.Background(),
			window: specification.RefundWindow{
				OriginalPostedAt: posted,
				Now:              posted.Add(31 * 24 * time.Hour),
				Window:           30 * 24 * time.Hour,
			},
			expectedPassed:        false,
			expectedViolationCode: "REFUND_WINDOW_EXPIRED",
		},
		{
			name:                  "zero candidate rejected without panic",
			ctx:                   context.Background(),
			window:                specification.RefundWindow{},
			expectedPassed:        false,
			expectedViolationCode: "REFUND_WINDOW_EXPIRED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := specification.RefundWindowValid().Evaluate(tc.ctx, tc.window)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestRefundAmountValid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		amounts               specification.RefundAmounts
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "exact remainder passes",
			ctx:  context.Background(),
			amounts: specification.RefundAmounts{
				OriginalMinor:        100,
				PreviousRefundsMinor: 40,
				RequestedMinor:       60,
				Asset:                testUSD,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "exceeding unrefunded remainder rejected",
			ctx:  context.Background(),
			amounts: specification.RefundAmounts{
				OriginalMinor:        100,
				PreviousRefundsMinor: 40,
				RequestedMinor:       61,
				Asset:                testUSD,
			},
			expectedPassed:        false,
			expectedViolationCode: "REFUND_EXCEEDS_ORIGINAL",
		},
		{
			name: "over-refunded history rejected",
			ctx:  context.Background(),
			amounts: specification.RefundAmounts{
				OriginalMinor:        100,
				PreviousRefundsMinor: 120,
				RequestedMinor:       1,
				Asset:                testUSD,
			},
			expectedPassed:        false,
			expectedViolationCode: "REFUND_EXCEEDS_ORIGINAL",
		},
		{
			name: "negative requested amount rejected",
			ctx:  context.Background(),
			amounts: specification.RefundAmounts{
				OriginalMinor:        100,
				PreviousRefundsMinor: 0,
				RequestedMinor:       -1,
				Asset:                testUSD,
			},
			expectedPassed:        false,
			expectedViolationCode: "REFUND_EXCEEDS_ORIGINAL",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := specification.RefundAmountValid().Evaluate(tc.ctx, tc.amounts)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}
