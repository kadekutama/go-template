package fixtures_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/test/fixtures"
)

func TestTenantFixtureDeterministic(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		id             string
		expectedTenant fixtures.TenantFixture
	}

	testCases := []testCase{
		{
			name: "default tenant stable ids",
			id:   "",
			expectedTenant: fixtures.TenantFixture{
				TenantID: "tnt-test-01",
				LedgerID: "ldg-test-01",
				Name:     "Test Tenant 01",
				Region:   "local",
			},
		},
		{
			name: "explicit tenant id preserved",
			id:   "tnt-test-02",
			expectedTenant: fixtures.TenantFixture{
				TenantID: "tnt-test-02",
				LedgerID: "ldg-test-01",
				Name:     "Test Tenant 01",
				Region:   "local",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedTenant, fixtures.Tenant(tc.id))
		})
	}
}

func TestAccountFixturesChart(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenantID      string
		ledgerID      string
		expectedCount int
	}

	testCases := []testCase{
		{
			name:          "default chart covers three kinds times three assets",
			tenantID:      "",
			ledgerID:      "",
			expectedCount: 9,
		},
		{
			name:          "explicit scope preserved",
			tenantID:      "tnt-test-02",
			ledgerID:      "ldg-test-02",
			expectedCount: 9,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accounts := fixtures.Accounts(tc.tenantID, tc.ledgerID)
			assert.Len(t, accounts, tc.expectedCount)

			seen := make(map[string]bool)
			for _, acct := range accounts {
				assert.NotEmpty(t, acct.AccountID)
				assert.NotEmpty(t, acct.AssetCode)
				assert.False(t, seen[acct.AccountID], "duplicate account id %s", acct.AccountID)
				seen[acct.AccountID] = true
			}

			again := fixtures.Accounts(tc.tenantID, tc.ledgerID)
			assert.Equal(t, accounts, again)
		})
	}
}

func TestPostingFixturesBalanced(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		tenantID         string
		ledgerID         string
		expectedPostings int
	}

	testCases := []testCase{
		{
			name:             "four stable postings all balanced",
			tenantID:         "",
			ledgerID:         "",
			expectedPostings: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postings := fixtures.Postings(tc.tenantID, tc.ledgerID)
			assert.Len(t, postings, tc.expectedPostings)

			for _, posting := range postings {
				debits := int64(0)
				credits := int64(0)

				for _, line := range posting.Lines {
					assert.Positive(t, line.AmountMinor)

					switch line.Side {
					case "DEBIT":
						debits += line.AmountMinor
					case "CREDIT":
						credits += line.AmountMinor
					default:
						t.Fatalf("unknown side %s", line.Side)
					}
				}

				assert.Equal(t, debits, credits, "posting %s unbalanced", posting.PostingID)
			}

			assert.Equal(t, postings, fixtures.Postings(tc.tenantID, tc.ledgerID))
		})
	}
}

func TestWorkflowAndStatementFixtures(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		tenantID           string
		expectedWorkflows  int
		expectedStatements int
	}

	testCases := []testCase{
		{
			name:               "workflows and statements deterministic",
			tenantID:           "",
			expectedWorkflows:  3,
			expectedStatements: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workflows := fixtures.Workflows(tc.tenantID)
			assert.Len(t, workflows, tc.expectedWorkflows)

			statements := fixtures.Statements()
			assert.Len(t, statements, tc.expectedStatements)

			assert.Equal(t, workflows, fixtures.Workflows(tc.tenantID))
			assert.Equal(t, statements, fixtures.Statements())
		})
	}
}
