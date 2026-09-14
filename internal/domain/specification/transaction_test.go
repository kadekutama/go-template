package specification_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func txEntries(pid valueobject.PostingID, dAmt, cAmt int64) []entity.Entry {
	return []entity.Entry{
		{ID: "e-1", PostingID: pid, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: dAmt, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-2", PostingID: pid, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: cAmt, AssetCode: testUSD, AccountSeq: 1},
	}
}

func txPosting(pid valueobject.PostingID, entries []entity.Entry) entity.PostingData {
	return entity.PostingData{
		ID:        pid,
		TenantID:  testTenant1,
		LedgerID:  testLedger1,
		Operation: "transfer.v1",
		Entries:   entries,
	}
}

func transferTemplate() specification.PostingTemplate {
	return specification.PostingTemplate{
		Operation: "transfer.v1",
		Version:   "v1",
		Rules: []specification.TemplateRule{
			{Class: valueobject.ClassLiability, Side: valueobject.DirectionDebit},
			{Class: valueobject.ClassLiability, Side: valueobject.DirectionCredit},
		},
	}
}

func templateAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: {ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "2001", Name: "src", Class: valueobject.ClassLiability, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
		testAccount2: {ID: testAccount2, TenantID: testTenant1, LedgerID: testLedger1, Number: "2002", Name: "dst", Class: valueobject.ClassLiability, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
	}
}

func TestEntryAmountPositive(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		entry                 entity.Entry
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "positive entry amount passes",
			ctx:                   context.Background(),
			entry:                 txEntries(testPosting1, 5, 5)[0],
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:                  "zero entry amount rejected",
			ctx:                   context.Background(),
			entry:                 txEntries(testPosting2, 0, 0)[0],
			expectedPassed:        false,
			expectedViolationCode: "INVALID_ENTRY_AMOUNT",
		},
		{
			name:                  "zero entry struct rejected without panic",
			ctx:                   context.Background(),
			entry:                 entity.Entry{},
			expectedPassed:        false,
			expectedViolationCode: "INVALID_ENTRY_AMOUNT",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.EntryAmountPositive().Evaluate(tc.ctx, tc.entry)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestPostingBalancesPerCurrency(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		posting               entity.PostingData
		expectedPassed        bool
		expectedViolationCode string
		expectedAssetDetail   string
	}

	testCases := []testCase{
		{
			name:                  "balanced single asset posting passes",
			ctx:                   context.Background(),
			posting:               txPosting(testPosting1, txEntries(testPosting1, 100, 100)),
			expectedPassed:        true,
			expectedViolationCode: "",
			expectedAssetDetail:   "",
		},
		{
			name:                  "imbalanced single asset posting rejected",
			ctx:                   context.Background(),
			posting:               txPosting(testPosting2, txEntries(testPosting2, 100, 90)),
			expectedPassed:        false,
			expectedViolationCode: "UNBALANCED_TRANSACTION",
			expectedAssetDetail:   testUSD,
		},
		{
			name: "multi-asset with broken EUR lot rejected",
			ctx:  context.Background(),
			posting: txPosting(
				testPosting3,
				append(
					txEntries(testPosting3, 100, 100),
					entity.Entry{ID: "e-3", PostingID: testPosting3, AccountID: testAccount3, Side: valueobject.DirectionDebit, AmountMinor: 50, AssetCode: testEUR, AccountSeq: 1},
					entity.Entry{ID: "e-4", PostingID: testPosting3, AccountID: "a-4", Side: valueobject.DirectionCredit, AmountMinor: 40, AssetCode: testEUR, AccountSeq: 1},
				),
			),
			expectedPassed:        false,
			expectedViolationCode: "UNBALANCED_TRANSACTION",
			expectedAssetDetail:   testEUR,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.PostingBalancesPerCurrency().Evaluate(tc.ctx, tc.posting)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
				if tc.expectedAssetDetail != "" {
					assert.Equal(t, tc.expectedAssetDetail, res.Violations[0].Details["asset"])
				}
			}
		})
	}
}

func TestPostingTemplateAllowed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		template              specification.PostingTemplate
		accounts              map[valueobject.AccountID]entity.AccountData
		posting               entity.PostingData
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "matching template rules pass",
			ctx:                   context.Background(),
			template:              transferTemplate(),
			accounts:              templateAccounts(),
			posting:               txPosting(testPosting1, txEntries(testPosting1, 100, 100)),
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:     "mismatched operation rejected",
			ctx:      context.Background(),
			template: transferTemplate(),
			accounts: templateAccounts(),
			posting: func() entity.PostingData {
				p := txPosting(testPosting2, txEntries(testPosting2, 100, 100))
				p.Operation = "payout.v1"
				return p
			}(),
			expectedPassed:        false,
			expectedViolationCode: "INVALID_POSTING_TEMPLATE",
		},
		{
			name:     "account class mismatch against template rule rejected",
			ctx:      context.Background(),
			template: transferTemplate(),
			accounts: func() map[valueobject.AccountID]entity.AccountData {
				accts := templateAccounts()
				accts[testAccount1] = entity.AccountData{
					ID:        testAccount1,
					TenantID:  testTenant1,
					LedgerID:  testLedger1,
					Number:    "1000",
					Name:      "cash",
					Class:     valueobject.ClassAsset,
					AssetCode: testUSD,
					Status:    valueobject.StatusActive,
					Version:   1,
				}
				return accts
			}(),
			posting:               txPosting(testPosting3, txEntries(testPosting3, 100, 100)),
			expectedPassed:        false,
			expectedViolationCode: "INVALID_POSTING_TEMPLATE",
		},
		{
			name:                  "unknown account rejected",
			ctx:                   context.Background(),
			template:              transferTemplate(),
			accounts:              map[valueobject.AccountID]entity.AccountData{},
			posting:               txPosting(testPosting4, txEntries(testPosting4, 100, 100)),
			expectedPassed:        false,
			expectedViolationCode: "INVALID_POSTING_TEMPLATE",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec := specification.PostingTemplateAllowed(tc.template, tc.accounts)
			res := spec.Evaluate(tc.ctx, tc.posting)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestValidCurrency(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		check                 specification.CurrencyCheck
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "same asset matches without fx",
			ctx:  context.Background(),
			check: specification.CurrencyCheck{
				EntryAsset:   testUSD,
				AccountAsset: testUSD,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "different assets without fx rejected",
			ctx:  context.Background(),
			check: specification.CurrencyCheck{
				EntryAsset:   testUSD,
				AccountAsset: testEUR,
			},
			expectedPassed:        false,
			expectedViolationCode: "CURRENCY_MISMATCH",
		},
		{
			name: "different assets with approved fx pass",
			ctx:  context.Background(),
			check: specification.CurrencyCheck{
				EntryAsset:   testUSD,
				AccountAsset: testEUR,
				FXApproved:   true,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.ValidCurrency().Evaluate(tc.ctx, tc.check)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestOriginalExists(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		ref                   specification.OriginalRef
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "found original posting passes",
			ctx:  context.Background(),
			ref: specification.OriginalRef{
				ID:    testPosting1,
				Found: true,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "missing original posting rejected",
			ctx:  context.Background(),
			ref: specification.OriginalRef{
				ID:    "p-9",
				Found: false,
			},
			expectedPassed:        false,
			expectedViolationCode: "ORIGINAL_NOT_FOUND",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.OriginalExists().Evaluate(tc.ctx, tc.ref)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}
