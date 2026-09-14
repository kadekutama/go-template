package valueobject_test

import (
	"math"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func testMulDivEdges(t *testing.T, m valueobject.Money) {
	t.Helper()
	zero, err := m.MulScalar(0)
	if err != nil || !zero.IsZero() {
		t.Fatalf("7*0 = %+v, %v", zero, err)
	}
	neg, err := m.MulScalar(-3)
	if err != nil || neg.AmountMinor() != -21 {
		t.Fatalf("7*-3 = %+v, %v", neg, err)
	}
	div, err := valueobject.MustMoney(7, testUSD).DivScalar(2)
	if err != nil || div.AmountMinor() != 3 {
		t.Fatalf("7/2 = %+v, %v (want truncating 3)", div, err)
	}
}

func testScalarOverflow(t *testing.T) {
	t.Helper()
	if _, err := valueobject.MustMoney(math.MinInt64, testUSD).DivScalar(-1); err == nil {
		t.Error("MinInt64/-1 must overflow")
	}
	if _, err := valueobject.MustMoney(math.MinInt64, testUSD).MulScalar(-1); err == nil {
		t.Error("MinInt64*-1 must overflow")
	}
	if _, err := valueobject.MustMoney(-1, testUSD).MulScalar(math.MinInt64); err == nil {
		t.Error("-1*MinInt64 must overflow")
	}
	if _, err := valueobject.MustMoney(math.MinInt64, testUSD).Sub(valueobject.MustMoney(1, testUSD)); err == nil {
		t.Error("MinInt64-1 must overflow")
	}
}

func TestMoneyScalarEdges(t *testing.T) {
	t.Parallel()
	m := valueobject.MustMoney(7, testUSD)
	testMulDivEdges(t, m)
	testScalarOverflow(t)
	if _, err := valueobject.NewMoney(1, ""); err == nil {
		t.Error("empty asset must error")
	}
	defer func() {
		if recover() == nil {
			t.Error("MustMoney with empty asset must panic")
		}
	}()
	valueobject.MustMoney(1, "")
}

func TestMoneyCompareAndPredicates(t *testing.T) {
	t.Parallel()
	a := valueobject.MustMoney(5, testUSD)
	b := valueobject.MustMoney(9, testUSD)
	aCopy := valueobject.MustMoney(5, testUSD)
	if c, _ := a.Compare(aCopy); c != 0 {
		t.Errorf("equal compare = %d, want 0", c)
	}
	if c, _ := a.Compare(b); c != -1 {
		t.Errorf("less compare = %d, want -1", c)
	}
	if c, _ := b.Compare(a); c != 1 {
		t.Errorf("greater compare = %d, want 1", c)
	}
	if !valueobject.MustMoney(0, testUSD).IsZero() || a.IsZero() {
		t.Error("IsZero wrong")
	}
	if !a.IsPositive() || valueobject.MustMoney(-1, testUSD).IsPositive() {
		t.Error("IsPositive wrong")
	}
	if a.Asset() != testUSD || a.AmountMinor() != 5 {
		t.Error("accessors wrong")
	}
}

func TestMoneyFormatNegative(t *testing.T) {
	t.Parallel()
	reg, _ := valueobject.NewRegistry(
		valueobject.AssetInfo{Code: testUSD, Exponent: 2, Kind: valueobject.AssetKindFiat},
		valueobject.AssetInfo{Code: "JPY", Exponent: 0, Kind: valueobject.AssetKindFiat},
	)
	s, err := valueobject.MustMoney(-1050, testUSD).Format(reg)
	if err != nil || s != "-10.50" {
		t.Errorf("Format(-1050) = %q, %v", s, err)
	}
	s, err = valueobject.MustMoney(-5, "JPY").Format(reg)
	if err != nil || s != "-5" {
		t.Errorf("Format(-5 JPY) = %q, %v", s, err)
	}
	s, err = valueobject.MustMoney(math.MinInt64, testUSD).Format(reg)
	if err != nil || s != "-92233720368547758.08" {
		t.Errorf("Format(MinInt64) = %q, %v", s, err)
	}
}

func TestAllocateEdges(t *testing.T) {
	t.Parallel()
	// Negative total mirrors sign onto exact-sum shares.
	shares, err := valueobject.AllocateLargestRemainder(-100, []int64{1, 1, 1})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	var sum int64
	for _, s := range shares {
		sum += s
	}
	if sum != -100 || shares[0] != -34 {
		t.Fatalf("negative split = %v", shares)
	}
	// All-zero weights split evenly.
	shares, err = valueobject.AllocateLargestRemainder(10, []int64{0, 0})
	if err != nil {
		t.Fatalf("Allocate: %v", err)
	}
	if shares[0] != 5 || shares[1] != 5 {
		t.Fatalf("zero-weight split = %v, want [5 5]", shares)
	}
	// Overflow in weight*total products.
	if _, err := valueobject.AllocateLargestRemainder(math.MaxInt64, []int64{math.MaxInt64}); err == nil {
		t.Error("overflowing allocation must error")
	}
	// Weight-sum overflow.
	if _, err := valueobject.AllocateLargestRemainder(1, []int64{math.MaxInt64, math.MaxInt64}); err == nil {
		t.Error("overflowing weight sum must error")
	}
}

func TestRegistryZeroValue(t *testing.T) {
	t.Parallel()
	var reg valueobject.Registry
	if err := reg.Register(valueobject.AssetInfo{Code: "USD", Exponent: 2, Kind: valueobject.AssetKindFiat}); err != nil {
		t.Fatalf("Register on zero Registry: %v", err)
	}
	if _, err := reg.Lookup("USD"); err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if _, err := valueobject.NewRegistry(valueobject.AssetInfo{Code: "", Exponent: 0}); err == nil {
		t.Error("NewRegistry with empty code must error")
	}
}

func TestAllIDTypesTable(t *testing.T) {
	t.Parallel()
	type idCase struct {
		name  string
		parse func(string) (string, error)
	}
	cases := []idCase{
		{kindAccount, func(s string) (string, error) {
			id, err := valueobject.ParseAccountID(s)
			return id.String(), err
		}},
		{kindPosting, func(s string) (string, error) {
			id, err := valueobject.ParsePostingID(s)
			return id.String(), err
		}},
		{kindEntry, func(s string) (string, error) {
			id, err := valueobject.ParseEntryID(s)
			return id.String(), err
		}},
		{kindHold, func(s string) (string, error) {
			id, err := valueobject.ParseHoldID(s)
			return id.String(), err
		}},
		{kindTenant, func(s string) (string, error) {
			id, err := valueobject.ParseTenantID(s)
			return id.String(), err
		}},
		{kindLedger, func(s string) (string, error) {
			id, err := valueobject.ParseLedgerID(s)
			return id.String(), err
		}},
		{kindUser, func(s string) (string, error) {
			id, err := valueobject.ParseUserID(s)
			return id.String(), err
		}},
		{kindJournal, func(s string) (string, error) {
			id, err := valueobject.ParseJournalID(s)
			return id.String(), err
		}},
		{kindPeriod, func(s string) (string, error) {
			id, err := valueobject.ParsePeriodID(s)
			return id.String(), err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gen := &seqIDs{}
			a, b := gen.NewID(), gen.NewID()
			if a == b {
				t.Fatal("generated ids must be unique")
			}
			back, err := tc.parse(a)
			if err != nil || back != a {
				t.Fatalf("round-trip = %q, %v", back, err)
			}
			if _, err := tc.parse("0193e5f0-1c2c-7a4e-b3f2"); err == nil {
				t.Error("truncated uuid must error")
			}
			if _, err := tc.parse("0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1!"); err == nil {
				t.Error("non-hex uuid must error")
			}
			if _, err := tc.parse("0193e5f01c2c-7a4e-b3f2-9c1e5a7b9c1e"); err == nil {
				t.Error("misplaced hyphen must error")
			}
			if _, err := tc.parse("0193E5F0-1C2C-7A4E-B3F2-9C1E5A7B9C1E"); err != nil {
				t.Errorf("uppercase hex must parse: %v", err)
			}
			if _, err := tc.parse("a-b-c-d-e-f"); err == nil {
				t.Error("six groups must error")
			}
		})
	}
}

func checkIDEquivalence(t *testing.T, s1, s2 string) {
	t.Helper()
	a1, _ := valueobject.ParseAccountID(s1)
	a1Copy, _ := valueobject.ParseAccountID(s1)
	a2, _ := valueobject.ParseAccountID(s2)
	if !a1.Equals(a1Copy) || a1.Equals(a2) {
		t.Error("account id equality failed")
	}

	p1, _ := valueobject.ParsePostingID(s1)
	p1Copy, _ := valueobject.ParsePostingID(s1)
	p2, _ := valueobject.ParsePostingID(s2)
	if !p1.Equals(p1Copy) || p1.Equals(p2) {
		t.Error("posting id equality failed")
	}

	e1, _ := valueobject.ParseEntryID(s1)
	e1Copy, _ := valueobject.ParseEntryID(s1)
	e2, _ := valueobject.ParseEntryID(s2)
	if !e1.Equals(e1Copy) || e1.Equals(e2) {
		t.Error("entry id equality failed")
	}

	h1, _ := valueobject.ParseHoldID(s1)
	h1Copy, _ := valueobject.ParseHoldID(s1)
	h2, _ := valueobject.ParseHoldID(s2)
	if !h1.Equals(h1Copy) || h1.Equals(h2) {
		t.Error("hold id equality failed")
	}
}

func checkScopeIDEquivalence(t *testing.T, s1, s2 string) {
	t.Helper()
	t1, _ := valueobject.ParseTenantID(s1)
	t1Copy, _ := valueobject.ParseTenantID(s1)
	t2, _ := valueobject.ParseTenantID(s2)
	if !t1.Equals(t1Copy) || t1.Equals(t2) {
		t.Error("tenant id equality failed")
	}

	l1, _ := valueobject.ParseLedgerID(s1)
	l1Copy, _ := valueobject.ParseLedgerID(s1)
	l2, _ := valueobject.ParseLedgerID(s2)
	if !l1.Equals(l1Copy) || l1.Equals(l2) {
		t.Error("ledger id equality failed")
	}

	u1, _ := valueobject.ParseUserID(s1)
	u1Copy, _ := valueobject.ParseUserID(s1)
	u2, _ := valueobject.ParseUserID(s2)
	if !u1.Equals(u1Copy) || u1.Equals(u2) {
		t.Error("user id equality failed")
	}

	j1, _ := valueobject.ParseJournalID(s1)
	j1Copy, _ := valueobject.ParseJournalID(s1)
	j2, _ := valueobject.ParseJournalID(s2)
	if !j1.Equals(j1Copy) || j1.Equals(j2) {
		t.Error("journal id equality failed")
	}

	d1, _ := valueobject.ParsePeriodID(s1)
	d1Copy, _ := valueobject.ParsePeriodID(s1)
	d2, _ := valueobject.ParsePeriodID(s2)
	if !d1.Equals(d1Copy) || d1.Equals(d2) {
		t.Error("period id equality failed")
	}
}

func TestIDEqualsAllTypes(t *testing.T) {
	t.Parallel()
	gen := &seqIDs{}
	s1, s2 := gen.NewID(), gen.NewID()
	checkIDEquivalence(t, s1, s2)
	checkScopeIDEquivalence(t, s1, s2)
	if _, err := valueobject.ParseHoldID("bad"); err == nil {
		t.Error("bad hold id must error")
	}
}

func TestParseRejectsAlternateUUIDForms(t *testing.T) {
	t.Parallel()
	// Stdlib uuid.Parse accepts these; the domain contract is strict canonical.
	for _, s := range []string{
		"urn:uuid:0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1e",
		"{0193e5f0-1c2c-7a4e-b3f2-9c1e5a7b9c1e}",
		"0193e5f01c2c7a4eb3f29c1e5a7b9c1e",
	} {
		if _, err := valueobject.ParseAccountID(s); err == nil {
			t.Errorf("ParseAccountID(%q) must reject non-canonical form", s)
		}
	}
}
