package valueobject_test

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func usd(t *testing.T, minor int64) valueobject.Money {
	t.Helper()
	m, err := valueobject.NewMoney(minor, testUSD)
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	return m
}

func TestTenthPlusFifthEqualsThreeTenths(t *testing.T) {
	t.Parallel()
	a := usd(t, 10)
	b := usd(t, 20)
	sum, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if sum.AmountMinor() != 30 {
		t.Fatalf("0.1+0.2 = %d minor, want 30", sum.AmountMinor())
	}
}

func TestCrossCurrencyMismatch(t *testing.T) {
	t.Parallel()
	a := usd(t, 100)
	e, err := valueobject.NewMoney(50, "EUR")
	if err != nil {
		t.Fatalf("NewMoney: %v", err)
	}
	if _, err := a.Add(e); err == nil || !strings.Contains(err.Error(), "CURRENCY_MISMATCH") {
		t.Errorf("Add cross-currency err = %v, want CURRENCY_MISMATCH", err)
	}
	if _, err := a.Sub(e); err == nil || !strings.Contains(err.Error(), "CURRENCY_MISMATCH") {
		t.Errorf("Sub cross-currency err = %v, want CURRENCY_MISMATCH", err)
	}
	if _, err := a.Compare(e); err == nil || !strings.Contains(err.Error(), "CURRENCY_MISMATCH") {
		t.Errorf("Compare cross-currency err = %v, want CURRENCY_MISMATCH", err)
	}
}

func TestOverflowRejected(t *testing.T) {
	t.Parallel()
	max := usd(t, math.MaxInt64)
	if _, err := max.Add(usd(t, 1)); err == nil {
		t.Error("MaxInt64+1 must overflow")
	}
	if _, err := max.MulScalar(2); err == nil {
		t.Error("MaxInt64*2 must overflow")
	}
	if _, err := usd(t, math.MinInt64).Sub(usd(t, 1)); err == nil {
		t.Error("MinInt64-1 must overflow")
	}
	if _, err := usd(t, 1).DivScalar(0); err == nil {
		t.Error("division by zero must error")
	}
}

func TestArithmeticProperties(t *testing.T) {
	t.Parallel()
	// #nosec G404 -- deterministic pseudo-random generator for property testing
	rng := rand.New(rand.NewPCG(42, 100))
	vals := make([]int64, 0, 64)
	for i := 0; i < 64; i++ {
		vals = append(vals, rng.Int64N(1_000_000_000)-500_000_000)
	}
	for i := 0; i < 1000; i++ {
		a, b, c := vals[rng.IntN(len(vals))], vals[rng.IntN(len(vals))], vals[rng.IntN(len(vals))]
		ma, mb, mc := usd(t, a), usd(t, b), usd(t, c)
		ab, _ := ma.Add(mb)
		abc1, _ := ab.Add(mc)
		bc, _ := mb.Add(mc)
		abc2, _ := ma.Add(bc)
		if abc1.AmountMinor() != abc2.AmountMinor() {
			t.Fatalf("associativity broke for %d %d %d", a, b, c)
		}
		ba, _ := mb.Add(ma)
		if ab.AmountMinor() != ba.AmountMinor() {
			t.Fatalf("commutativity broke for %d %d", a, b)
		}
	}
}

func TestMoneyJSONShape(t *testing.T) {
	t.Parallel()
	// The domain owns the canonical shape; byte marshaling happens outside
	// (test-only encoding/json use is exempt from the E01-T07 codec seam,
	// which checks non-test imports).
	data, err := json.Marshal(usd(t, 1050).DTO())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"amount_minor":1050`) || !strings.Contains(string(data), `"asset_code":"`+testUSD+`"`) {
		t.Fatalf("unexpected json: %s", data)
	}
	var dto valueobject.MoneyDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	back, err := valueobject.MoneyFromDTO(dto)
	if err != nil {
		t.Fatalf("MoneyFromDTO: %v", err)
	}
	if back.AmountMinor() != 1050 || back.Asset() != testUSD {
		t.Fatalf("round-trip = %+v", back)
	}
	var bad valueobject.MoneyDTO
	if err := json.Unmarshal([]byte(`{"amount_minor":10.5,"asset_code":"`+testUSD+`"}`), &bad); err == nil {
		t.Error("float quantity must not decode into int64")
	}
	if _, err := valueobject.MoneyFromDTO(valueobject.MoneyDTO{AmountMinor: 10}); err == nil {
		t.Error("empty asset must error")
	}
}

func TestMoneyFormat(t *testing.T) {
	t.Parallel()
	reg, err := valueobject.NewRegistry(
		valueobject.AssetInfo{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
		valueobject.AssetInfo{Code: "JPY", Exponent: 0, Kind: valueobject.AssetKindFiat},
	)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	s, err := usd(t, 1050).Format(reg)
	if err != nil || s != "10.50" {
		t.Errorf("Format = %q, %v; want 10.50", s, err)
	}
	s, err = valueobject.MustMoney(100, "JPY").Format(reg)
	if err != nil || s != "100" {
		t.Errorf("Format JPY = %q, %v; want 100", s, err)
	}
	if _, err := valueobject.MustMoney(1, "EUR").Format(reg); err == nil {
		t.Error("Format of unregistered code must error")
	}
	_ = math.MaxInt64
}

func TestAllocateLargestRemainder(t *testing.T) {
	t.Parallel()
	shares, err := valueobject.AllocateLargestRemainder(100, []int64{50, 30, 20})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	var sum int64
	for _, s := range shares {
		sum += s
	}
	if sum != 100 {
		t.Fatalf("shares %v sum to %d, want 100", shares, sum)
	}
	// 100 split 3 ways: 34/33/33 with remainder to the first (tie → lowest index).
	shares, err = valueobject.AllocateLargestRemainder(100, []int64{1, 1, 1})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if !slices.Equal(shares, []int64{34, 33, 33}) {
		t.Fatalf("equal split = %v, want [34 33 33]", shares)
	}
	// Hare-Niemeyer ranking by fractional remainders with unequal weights:
	// Total: 11, Weights: [100, 300, 200], Sum: 600.
	// Initial integer quotients: [1, 5, 3] (sum 9, leftover 2).
	// Remainders: [500, 300, 400] / 600.
	// Highest remainders: index 0 (500) and index 2 (400) receive +1 each -> [2, 5, 4].
	shares, err = valueobject.AllocateLargestRemainder(11, []int64{100, 300, 200})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if !slices.Equal(shares, []int64{2, 5, 4}) {
		t.Fatalf("Hare-Niemeyer split = %v, want [2 5 4]", shares)
	}
	// Total: 10, Weights: [1, 2, 3], Sum: 6.
	// Initial quotients: [1, 3, 5] (sum 9, leftover 1).
	// Remainders: [4, 2, 0].
	// Highest remainder: index 0 (4) receives +1 -> [2, 3, 5].
	shares, err = valueobject.AllocateLargestRemainder(10, []int64{1, 2, 3})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if !slices.Equal(shares, []int64{2, 3, 5}) {
		t.Fatalf("Hare-Niemeyer split = %v, want [2 3 5]", shares)
	}
	for _, bad := range []struct {
		total   int64
		weights []int64
	}{
		{100, nil},
		{100, []int64{}},
		{100, []int64{1, -1}},
		{math.MinInt64, []int64{1, 1}},
	} {
		if _, err := valueobject.AllocateLargestRemainder(bad.total, bad.weights); err == nil {
			t.Errorf("Allocate(%d, %v) must error", bad.total, bad.weights)
		}
	}
}
