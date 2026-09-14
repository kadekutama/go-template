package valueobject_test

import (
	"testing"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseDirection(t *testing.T) {
	t.Parallel()
	if d, err := valueobject.ParseDirection("DEBIT"); err != nil || d != valueobject.DirectionDebit {
		t.Errorf("ParseDirection(DEBIT) = %q, %v", d, err)
	}
	if d, err := valueobject.ParseDirection("CREDIT"); err != nil || d != valueobject.DirectionCredit {
		t.Errorf("ParseDirection(CREDIT) = %q, %v", d, err)
	}
	if _, err := valueobject.ParseDirection("SIDEWAYS"); err == nil {
		t.Error("invalid direction must error")
	}
}

func TestParseAccountClass(t *testing.T) {
	t.Parallel()
	for _, c := range []string{"ASSET", "LIABILITY", "EQUITY", "REVENUE", "EXPENSE"} {
		if _, err := valueobject.ParseAccountClass(c); err != nil {
			t.Errorf("ParseAccountClass(%s): %v", c, err)
		}
	}
	if _, err := valueobject.ParseAccountClass("CRYPTO"); err == nil {
		t.Error("invalid class must error")
	}
}

func TestNormalSide(t *testing.T) {
	t.Parallel()
	cases := map[valueobject.AccountClass]valueobject.Direction{
		valueobject.ClassAsset:     valueobject.DirectionDebit,
		valueobject.ClassExpense:   valueobject.DirectionDebit,
		valueobject.ClassLiability: valueobject.DirectionCredit,
		valueobject.ClassEquity:    valueobject.DirectionCredit,
		valueobject.ClassRevenue:   valueobject.DirectionCredit,
	}
	for class, want := range cases {
		if got := class.NormalSide(); got != want {
			t.Errorf("%s.NormalSide = %s, want %s", class, got, want)
		}
	}
}

func TestParseAccountStatus(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"ACTIVE", "FROZEN", "CLOSED"} {
		if _, err := valueobject.ParseAccountStatus(s); err != nil {
			t.Errorf("ParseAccountStatus(%s): %v", s, err)
		}
	}
	if _, err := valueobject.ParseAccountStatus("PENDING"); err == nil {
		t.Error("invalid status must error")
	}
}
