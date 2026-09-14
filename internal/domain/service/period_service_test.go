package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD      = "USD"
	testEUR      = "EUR"
	testTenant1  = "t-1"
	testTenant2  = "t-2"
	testLedger1  = "l-1"
	testAccount1 = "a-1"
	testAccount2 = "a-2"
)

func periodAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: {ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "1000", Name: "usd", Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
		testAccount2: {ID: testAccount2, TenantID: testTenant1, LedgerID: testLedger1, Number: "3000", Name: "eur", Class: valueobject.ClassAsset, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1},
	}
}

func TestBelongsToSubLedger(t *testing.T) {
	t.Parallel()

	accounts := periodAccounts()
	usdEntry := entity.Entry{
		ID:          "e-1",
		PostingID:   "p-1",
		AccountID:   testAccount1,
		Side:        valueobject.DirectionDebit,
		AmountMinor: 10,
		AssetCode:   testUSD,
		AccountSeq:  1,
	}
	usdKey := service.Key(testTenant1, testLedger1, testUSD)
	eurKey := service.Key(testTenant1, testLedger1, testEUR)

	type testCase struct {
		name           string
		entry          entity.Entry
		accounts       map[valueobject.AccountID]entity.AccountData
		key            service.SubLedgerKey
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "matching sub-ledger key and asset",
			entry:          usdEntry,
			accounts:       accounts,
			key:            usdKey,
			expectedResult: true,
		},
		{
			name:           "differing asset key (EUR vs USD entry)",
			entry:          usdEntry,
			accounts:       accounts,
			key:            eurKey,
			expectedResult: false,
		},
		{
			name:           "differing tenant key",
			entry:          usdEntry,
			accounts:       accounts,
			key:            service.Key(testTenant2, testLedger1, testUSD),
			expectedResult: false,
		},
		{
			name: "unknown account in lookup",
			entry: entity.Entry{
				AccountID: "ghost",
			},
			accounts:       accounts,
			key:            usdKey,
			expectedResult: false,
		},
		{
			name: "entry and account asset mismatch",
			entry: func() entity.Entry {
				e := usdEntry
				e.AssetCode = testEUR
				return e
			}(),
			accounts:       accounts,
			key:            eurKey,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.BelongsToSubLedger(tc.entry, tc.accounts, tc.key)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestSubLedgerKeyMatches(t *testing.T) {
	t.Parallel()

	usdKey := service.Key(testTenant1, testLedger1, testUSD)

	type testCase struct {
		name           string
		key            service.SubLedgerKey
		tenant         valueobject.TenantID
		ledger         valueobject.LedgerID
		asset          valueobject.AssetCode
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "exact match",
			key:            usdKey,
			tenant:         testTenant1,
			ledger:         testLedger1,
			asset:          testUSD,
			expectedResult: true,
		},
		{
			name:           "asset mismatch",
			key:            usdKey,
			tenant:         testTenant1,
			ledger:         testLedger1,
			asset:          testEUR,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := tc.key.Matches(tc.tenant, tc.ledger, tc.asset)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func openPeriodData() entity.PeriodData {
	return entity.PeriodData{
		ID:       "pd-1",
		TenantID: testTenant1,
		LedgerID: testLedger1,
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC",
		Status:   entity.PeriodOpen,
		Version:  1,
	}
}

func openingLines() []service.OpeningBalanceLine {
	return []service.OpeningBalanceLine{
		{AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD},
		{AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD},
	}
}

func openingEvidence() service.EvidenceRef {
	return service.EvidenceRef{Source: "migration", URI: "s3://ledger/opening.csv", ApprovedBy: "cfo"}
}

func TestValidateOpeningBalance(t *testing.T) {
	t.Parallel()

	basePeriod := openPeriodData()
	baseLines := openingLines()
	baseEv := openingEvidence()

	type testCase struct {
		name          string
		period        entity.PeriodData
		lines         []service.OpeningBalanceLine
		ev            service.EvidenceRef
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid opening balance import",
			period:        basePeriod,
			lines:         baseLines,
			ev:            baseEv,
			expectedError: nil,
		},
		{
			name: "closed period rejected",
			period: func() entity.PeriodData {
				p := basePeriod
				p.Status = entity.PeriodClosed
				return p
			}(),
			lines:         baseLines,
			ev:            baseEv,
			expectedError: entity.NewError("PERIOD_CLOSED", "opening balances require an open period"),
		},
		{
			name:   "missing evidence reference",
			period: basePeriod,
			lines:  baseLines,
			ev:     service.EvidenceRef{},
			expectedError: entity.NewError(
				"EVIDENCE_REQUIRED",
				"opening balance requires source, evidence URI, and approver",
			),
		},
		{
			name:   "unbalanced debit and credit totals",
			period: basePeriod,
			lines: func() []service.OpeningBalanceLine {
				l := append([]service.OpeningBalanceLine(nil), baseLines...)
				l[1].AmountMinor = 90
				return l
			}(),
			ev:            baseEv,
			expectedError: entity.Errorf("UNBALANCED_TRANSACTION", "opening lines unbalanced for asset USD"),
		},
		{
			name:          "empty opening balance lines",
			period:        basePeriod,
			lines:         nil,
			ev:            baseEv,
			expectedError: entity.NewError("OPENING_LINES_REQUIRED", "opening balance requires at least one line"),
		},
		{
			name:   "zero line amount",
			period: basePeriod,
			lines: func() []service.OpeningBalanceLine {
				l := append([]service.OpeningBalanceLine(nil), baseLines...)
				l[0].AmountMinor = 0
				return l
			}(),
			ev:            baseEv,
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "opening line amount must be positive"),
		},
		{
			name:   "invalid side",
			period: basePeriod,
			lines: func() []service.OpeningBalanceLine {
				l := append([]service.OpeningBalanceLine(nil), baseLines...)
				l[0].Side = "SIDEWAYS"
				return l
			}(),
			ev:            baseEv,
			expectedError: entity.NewError("OPENING_SIDE_INVALID", "opening line side must be DEBIT or CREDIT"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualError := service.ValidateOpeningBalance(tc.period, tc.lines, tc.ev)
			assert.Equal(t, tc.expectedError, actualError)
		})
	}
}
