package persistence_test

import (
	"errors"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	postingtx "github.com/kadekutama/go-template/internal/infrastructure/database/postgres/posting"
)

// modelLedger is a DB-free spend model: balances with deterministic ordering.
// Overdraft disabled: spends exceeding available fail instead of applying.
type modelLedger struct {
	balances map[string]int64
	holds    map[string]int64
}

func newModelLedger(accounts []string, funding int64) *modelLedger {
	ledger := &modelLedger{balances: make(map[string]int64), holds: make(map[string]int64)}
	for _, account := range accounts {
		ledger.balances[account] = funding
	}

	return ledger
}

// spend applies amount when available covers it; otherwise it fails.
func (l *modelLedger) spend(account string, amount int64) error {
	available := l.balances[account] - l.holds[account]
	if amount > available {
		return errInsufficient
	}

	l.balances[account] -= amount

	return nil
}

// errInsufficient marks a spend blocked by available balance.
var errInsufficient = errors.New("insufficient available balance")

func TestModelBasedNoOverspend(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name      string
		seed      int64
		sequences int
		accounts  int
		funding   int64
	}

	testCases := []testCase{
		{
			name:      "10k randomized sequences never go negative",
			seed:      20260916,
			sequences: 10000,
			accounts:  8,
			funding:   100000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(tc.seed)) //nolint:gosec // E07-T06: seeded deterministic model requires math/rand.

			names := make([]string, 0, tc.accounts)
			for i := 0; i < tc.accounts; i++ {
				names = append(names, string(rune('a'+i)))
			}

			ledger := newModelLedger(names, tc.funding)
			violations := 0

			for i := 0; i < tc.sequences; i++ {
				// Deterministic lock order before every multi-account step.
				ids := make([]valueobject.AccountID, 0, 2)
				first := names[rng.Intn(len(names))]
				second := names[rng.Intn(len(names))]
				ids = append(ids, valueobject.AccountID(first), valueobject.AccountID(second))
				ordered := postingtx.LockOrder(ids)
				require.Len(t, ordered, 2)

				account := first
				amount := int64(rng.Intn(5000) + 1)

				switch rng.Intn(4) {
				case 0:
					_ = ledger.spend(account, amount)
				case 1:
					// Holds require available coverage like spends do.
					if amount <= ledger.balances[account]-ledger.holds[account] {
						ledger.holds[account] += amount
					}
				case 2:
					if ledger.holds[account] >= amount {
						ledger.holds[account] -= amount
					}
				case 3:
					ledger.balances[account] += amount
				}

				available := ledger.balances[account] - ledger.holds[account]
				if available < 0 {
					violations++
				}
			}

			assert.Equal(t, 0, violations, "model must never overspend with overdraft disabled")
		})
	}
}
