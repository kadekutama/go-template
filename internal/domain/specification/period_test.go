package specification_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestPeriodOpen(t *testing.T) {
	t.Parallel()

	open := entity.PeriodData{
		ID:       "pd-1",
		TenantID: testTenant1,
		LedgerID: testLedger1,
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}

	type testCase struct {
		name                  string
		ctx                   context.Context
		period                entity.PeriodData
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "open period passes",
			ctx:                   context.Background(),
			period:                open,
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "closed period rejected",
			ctx:  context.Background(),
			period: func() entity.PeriodData {
				p := open
				p.Status = entity.PeriodClosed
				return p
			}(),
			expectedPassed:        false,
			expectedViolationCode: "PERIOD_CLOSED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := specification.PeriodOpen().Evaluate(tc.ctx, tc.period)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestSpecComposition(t *testing.T) {
	t.Parallel()

	combined := specification.All[entity.PostingData](
		specification.PostingBalancesPerCurrency(),
		specification.PostingTemplateAllowed(transferTemplate(), templateAccounts()),
	)

	type testCase struct {
		name                   string
		ctx                    context.Context
		posting                entity.PostingData
		expectedPassed         bool
		expectedViolationCount int
	}

	testCases := []testCase{
		{
			name:                   "balanced posting with allowed template passes",
			ctx:                    context.Background(),
			posting:                txPosting(testPosting1, txEntries(testPosting1, 100, 100)),
			expectedPassed:         true,
			expectedViolationCount: 0,
		},
		{
			name: "imbalanced and wrong template aggregates two violations",
			ctx:  context.Background(),
			posting: func() entity.PostingData {
				p := txPosting(testPosting2, txEntries(testPosting2, 100, 90))
				p.Operation = "payout.v1"
				return p
			}(),
			expectedPassed:         false,
			expectedViolationCount: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res := combined.Evaluate(tc.ctx, tc.posting)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			assert.Len(t, res.Violations, tc.expectedViolationCount)
		})
	}
}
