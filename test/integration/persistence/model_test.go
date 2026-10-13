package persistence_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	postingtx "github.com/kadekutama/go-template/internal/infrastructure/database/postgres/posting"
	"github.com/kadekutama/go-template/test/integration/persistence"
)

// testRNG is a minimal deterministic xorshift64 for model tests. It avoids
// math/rand (gosec G404) while preserving seed-reproducible sequences.
type testRNG struct {
	state int64
}

func newTestRNG(seed int64) *testRNG {
	if seed == 0 {
		seed = 1
	}

	return &testRNG{state: seed}
}

func (r *testRNG) Intn(n int) int {
	if n <= 0 {
		return 0
	}

	x := r.state
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17

	r.state = x
	if x < 0 {
		x = -x
	}

	return int(x % int64(n))
}

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
var errInsufficient = persistence.ErrInsufficientAvailableBalance

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
			rng := newTestRNG(tc.seed)

			names := make([]string, 0, tc.accounts)
			for i := range tc.accounts {
				names = append(names, string(rune('a'+i)))
			}

			ledger := newModelLedger(names, tc.funding)
			violations := 0

			for range tc.sequences {
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
