package seed_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/database/seed"
	"github.com/kadekutama/go-template/test/fixtures"
)

// memoryStores is an idempotency-proving fake: Apply skips existing IDs.
type memoryStores struct {
	ledgers  map[string]seed.LedgerSeed
	accounts map[string]seed.AccountSeed
	postings map[string]seed.PostingSeed
}

func newMemoryStores() *memoryStores {
	return &memoryStores{
		ledgers:  make(map[string]seed.LedgerSeed),
		accounts: make(map[string]seed.AccountSeed),
		postings: make(map[string]seed.PostingSeed),
	}
}

type ledgerAdapter struct{ stores *memoryStores }

func (a ledgerAdapter) Exists(_ context.Context, id string) (bool, error) {
	_, ok := a.stores.ledgers[id]
	return ok, nil
}

func (a ledgerAdapter) Create(_ context.Context, ledger seed.LedgerSeed) error {
	a.stores.ledgers[ledger.ID] = ledger
	return nil
}

type accountAdapter struct{ stores *memoryStores }

func (a accountAdapter) Exists(_ context.Context, id string) (bool, error) {
	_, ok := a.stores.accounts[id]
	return ok, nil
}

func (a accountAdapter) Create(_ context.Context, account seed.AccountSeed) error {
	a.stores.accounts[account.ID] = account
	return nil
}

type postingAdapter struct{ stores *memoryStores }

func (a postingAdapter) Exists(_ context.Context, id string) (bool, error) {
	_, ok := a.stores.postings[id]
	return ok, nil
}

func (a postingAdapter) Create(_ context.Context, posting seed.PostingSeed) error {
	a.stores.postings[posting.ID] = posting
	return nil
}

func testStores(mem *memoryStores) seed.Stores {
	return seed.Stores{
		Ledgers:  ledgerAdapter{stores: mem},
		Accounts: accountAdapter{stores: mem},
		Postings: postingAdapter{stores: mem},
	}
}

func TestDevPlanShape(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		expectedLedgers  int
		expectedAccounts int
		expectedPostings int
	}

	testCases := []testCase{
		{
			name:             "dev plan covers chart and core patterns",
			expectedLedgers:  1,
			expectedAccounts: 9,
			expectedPostings: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := seed.DevPlan()
			assert.Len(t, plan.Ledgers, tc.expectedLedgers)
			assert.Len(t, plan.Accounts, tc.expectedAccounts)
			assert.Len(t, plan.Postings, tc.expectedPostings)
			assert.Equal(t, seed.TenantID, plan.Tenant.TenantID)

			operations := make(map[string]bool)
			for _, posting := range plan.Postings {
				operations[posting.Operation] = true

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

				assert.Equal(t, debits, credits, "posting %s unbalanced", posting.ID)
			}

			assert.True(t, operations["FUNDING"])
			assert.True(t, operations["TRANSFER"])
			assert.True(t, operations["FEE"])
			assert.True(t, operations["REFUND"])
		})
	}
}

func TestApplyIdempotent(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		runs             int
		expectedLedgers  int
		expectedAccounts int
		expectedPostings int
	}

	testCases := []testCase{
		{
			name:             "second apply yields the same row set",
			runs:             2,
			expectedLedgers:  1,
			expectedAccounts: 9,
			expectedPostings: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			plan := seed.DevPlan()
			mem := newMemoryStores()
			stores := testStores(mem)

			for i := 0; i < tc.runs; i++ {
				require.NoError(t, seed.Apply(ctx, plan, stores))
				assert.Len(t, mem.ledgers, tc.expectedLedgers)
				assert.Len(t, mem.accounts, tc.expectedAccounts)
				assert.Len(t, mem.postings, tc.expectedPostings)
			}
		})
	}
}

func TestApplyRequiresStores(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		plan          seed.Plan
		stores        seed.Stores
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil stores rejected",
			ctx:           context.Background(),
			plan:          seed.DevPlan(),
			stores:        seed.Stores{},
			expectedError: errors.New("seed: ledger, account, and posting stores are required"),
		},
		{
			name: "partial stores rejected",
			ctx:  context.Background(),
			plan: seed.DevPlan(),
			stores: seed.Stores{
				Ledgers: ledgerAdapter{stores: newMemoryStores()},
			},
			expectedError: errors.New("seed: ledger, account, and posting stores are required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := seed.Apply(tc.ctx, tc.plan, tc.stores)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}

func TestSeedMatchesFixtureIDs(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		expectedTenantID string
		expectedLedgerID string
	}

	testCases := []testCase{
		{
			name:             "seed and fixtures share one id universe",
			expectedTenantID: fixtures.DefaultTenantID,
			expectedLedgerID: fixtures.DefaultLedgerID,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan := seed.DevPlan()

			assert.Equal(t, tc.expectedTenantID, plan.Tenant.TenantID)
			assert.Equal(t, tc.expectedLedgerID, plan.Tenant.LedgerID)

			fixtureAccounts := make(map[string]bool)
			for _, account := range fixtures.Accounts(plan.Tenant.TenantID, plan.Tenant.LedgerID) {
				fixtureAccounts[account.AccountID] = true
			}

			for _, account := range plan.Accounts {
				assert.True(t, fixtureAccounts[account.ID], "account %s not in fixtures", account.ID)
			}

			fixturePostings := make(map[string]bool)
			for _, posting := range fixtures.Postings(plan.Tenant.TenantID, plan.Tenant.LedgerID) {
				fixturePostings[posting.PostingID] = true
			}

			for _, posting := range plan.Postings {
				assert.True(t, fixturePostings[posting.ID], "posting %s not in fixtures", posting.ID)
			}
		})
	}
}
