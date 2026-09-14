package domain_test

import (
	"math/rand/v2"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TestAllocateLargestRemainderExactness is a seeded property: random totals
// and weight vectors always produce shares summing exactly to the total.
func TestAllocateLargestRemainderExactness(t *testing.T) {
	t.Parallel()
	// #nosec G404 -- deterministic pseudo-random generator for property testing
	rng := rand.New(rand.NewPCG(20260914, 1))
	for i := 0; i < 1000; i++ {
		total := rng.Int64N(10_000_000) - 5_000_000
		n := 1 + rng.IntN(8)
		weights := make([]int64, n)
		for j := range weights {
			weights[j] = 1 + rng.Int64N(1000)
		}
		shares, err := valueobject.AllocateLargestRemainder(total, weights)
		if err != nil {
			t.Fatalf("case %d: Allocate(%d, %v): %v", i, total, weights, err)
		}
		var sum int64
		for _, s := range shares {
			if s < 0 && total >= 0 {
				t.Fatalf("case %d: negative share %d for non-negative total", i, s)
			}
			sum += s
		}
		if sum != total {
			t.Fatalf("case %d: shares %v sum to %d, want %d", i, shares, sum, total)
		}
	}
}

// TestMoneyAddAssociativitySeeded spot-checks exact integer arithmetic over
// random minor-unit triples (no precision loss by construction).
func TestMoneyAddAssociativitySeeded(t *testing.T) {
	t.Parallel()
	// #nosec G404 -- deterministic pseudo-random generator for property testing
	rng := rand.New(rand.NewPCG(7, 1))
	for i := 0; i < 500; i++ {
		vals := [3]int64{rng.Int64N(2_000_000) - 1_000_000, rng.Int64N(2_000_000) - 1_000_000, rng.Int64N(2_000_000) - 1_000_000}
		m := [3]valueobject.Money{
			valueobject.MustMoney(vals[0], testUSD),
			valueobject.MustMoney(vals[1], testUSD),
			valueobject.MustMoney(vals[2], testUSD),
		}
		ab, _ := m[0].Add(m[1])
		abc1, _ := ab.Add(m[2])
		bc, _ := m[1].Add(m[2])
		abc2, _ := m[0].Add(bc)
		if abc1.AmountMinor() != abc2.AmountMinor() {
			t.Fatalf("case %d: associativity broke", i)
		}
	}
}
