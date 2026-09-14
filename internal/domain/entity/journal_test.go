package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestNewJournal(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	mk := func(id, tenant, ledger string) entity.PostingData {
		return entity.PostingData{
			ID:        valueobject.PostingID(id),
			TenantID:  valueobject.TenantID(tenant),
			LedgerID:  valueobject.LedgerID(ledger),
			Operation: "transfer.v1",
			Entries: []entity.Entry{
				{ID: "e-1", PostingID: valueobject.PostingID(id), AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10, AssetCode: testUSD, AccountSeq: 1},
				{ID: "e-2", PostingID: valueobject.PostingID(id), AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 10, AssetCode: testUSD, AccountSeq: 1},
			},
			EffectiveAt: at,
			RecordedAt:  at,
		}
	}
	basePostings := []entity.PostingData{
		mk(testPosting1, testTenantID, testLedgerID),
		mk("p-2", testTenantID, testLedgerID),
	}

	type testCase struct {
		name          string
		id            valueobject.JournalID
		tenantID      valueobject.TenantID
		ledgerID      valueobject.LedgerID
		periodID      valueobject.PeriodID
		postings      []entity.PostingData
		nameField     string
		metadata      map[string]string
		createdAt     time.Time
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid journal creation",
			id:            testJournal1,
			tenantID:      testTenantID,
			ledgerID:      testLedgerID,
			periodID:      testPeriod1,
			postings:      basePostings,
			nameField:     "daily",
			metadata:      map[string]string{"k": "v"},
			createdAt:     at,
			expectedError: nil,
		},
		{
			name:     "mixed tenants among postings",
			id:       testJournal1,
			tenantID: testTenantID,
			ledgerID: testLedgerID,
			periodID: testPeriod1,
			postings: func() []entity.PostingData {
				p := append([]entity.PostingData(nil), basePostings...)
				p[1].TenantID = "t-2"
				return p
			}(),
			nameField:     "daily",
			metadata:      nil,
			createdAt:     at,
			expectedError: entity.NewError("JOURNAL_SCOPE_MISMATCH", "all postings must share the journal tenant and ledger"),
		},
		{
			name:     "mixed ledgers among postings",
			id:       testJournal1,
			tenantID: testTenantID,
			ledgerID: testLedgerID,
			periodID: testPeriod1,
			postings: func() []entity.PostingData {
				p := append([]entity.PostingData(nil), basePostings...)
				p[1].LedgerID = "l-2"
				return p
			}(),
			nameField:     "daily",
			metadata:      nil,
			createdAt:     at,
			expectedError: entity.NewError("JOURNAL_SCOPE_MISMATCH", "all postings must share the journal tenant and ledger"),
		},
		{
			name:          "empty postings",
			id:            testJournal1,
			tenantID:      testTenantID,
			ledgerID:      testLedgerID,
			periodID:      testPeriod1,
			postings:      nil,
			nameField:     "daily",
			metadata:      nil,
			createdAt:     at,
			expectedError: entity.NewError("JOURNAL_POSTINGS_REQUIRED", "journal requires at least one posting"),
		},
		{
			name:          "empty period id",
			id:            testJournal1,
			tenantID:      testTenantID,
			ledgerID:      testLedgerID,
			periodID:      "",
			postings:      basePostings,
			nameField:     "daily",
			metadata:      nil,
			createdAt:     at,
			expectedError: entity.NewError("JOURNAL_PERIOD_REQUIRED", "period id is required"),
		},
		{
			name:          "empty journal id",
			id:            "",
			tenantID:      testTenantID,
			ledgerID:      testLedgerID,
			periodID:      testPeriod1,
			postings:      basePostings,
			nameField:     "daily",
			metadata:      nil,
			createdAt:     at,
			expectedError: entity.NewError("JOURNAL_ID_REQUIRED", "journal id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			j, err := entity.NewJournal(tc.id, tc.tenantID, tc.ledgerID, tc.periodID, tc.postings, tc.nameField, tc.metadata, tc.createdAt)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, len(tc.postings), len(j.PostingIDs))
				assert.Equal(t, tc.periodID, j.PeriodID)
			}
		})
	}
}

func TestPeriodDataValidate(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	valid := entity.PeriodData{
		ID:       testPeriod1,
		TenantID: testTenantID,
		LedgerID: testLedgerID,
		Start:    start,
		End:      end,
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}

	type testCase struct {
		name          string
		period        entity.PeriodData
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid open period",
			period:        valid,
			expectedError: nil,
		},
		{
			name: "inverted start and end dates",
			period: func() entity.PeriodData {
				p := valid
				p.Start = end
				p.End = start
				return p
			}(),
			expectedError: entity.NewError("PERIOD_BOUNDS_INVALID", "period start must precede end"),
		},
		{
			name: "empty timezone",
			period: func() entity.PeriodData {
				p := valid
				p.Timezone = ""
				return p
			}(),
			expectedError: entity.NewError("PERIOD_TIMEZONE_REQUIRED", "accounting timezone is required"),
		},
		{
			name: "invalid period status",
			period: func() entity.PeriodData {
				p := valid
				p.Status = "AJAR"
				return p
			}(),
			expectedError: entity.NewError("PERIOD_STATUS_INVALID", "period status must be OPEN or CLOSED"),
		},
		{
			name: "zero version",
			period: func() entity.PeriodData {
				p := valid
				p.Version = 0
				return p
			}(),
			expectedError: entity.NewError("PERIOD_VERSION_INVALID", "version starts at 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.period.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestPeriodDataContains(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	period := entity.PeriodData{
		ID:       testPeriod1,
		TenantID: testTenantID,
		LedgerID: testLedgerID,
		Start:    start,
		End:      end,
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}

	type testCase struct {
		name           string
		period         entity.PeriodData
		t              time.Time
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "within period bounds",
			period:         period,
			t:              start.Add(time.Hour),
			expectedResult: true,
		},
		{
			name:           "at period end bound",
			period:         period,
			t:              end,
			expectedResult: false,
		},
		{
			name:           "before period start",
			period:         period,
			t:              start.Add(-time.Hour),
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.period.Contains(tc.t)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
