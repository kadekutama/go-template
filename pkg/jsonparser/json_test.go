package jsonparser

import (
	"strings"
	"testing"
)

// moneyShape mirrors ledger integer-minor amounts: exact int64 round-trip is
// money-safety relevant (no float encoding may sneak in).
type moneyShape struct {
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
}

func TestRoundTripMinorMoney(t *testing.T) {
	t.Parallel()

	in := moneyShape{AccountID: "acc_01", AmountMinor: 9_007_199_254_740_993, AssetCode: "USD"}
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "e+") || strings.Contains(string(data), "E+") {
		t.Fatalf("amount must not use exponent notation: %s", data)
	}
	var out moneyShape
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out != in {
		t.Errorf("round-trip mismatch: %+v != %+v", out, in)
	}
}

func TestRoundTripLargePayload(t *testing.T) {
	t.Parallel()

	in := make([]moneyShape, 0, 20000)
	for i := 0; i < 20000; i++ {
		in = append(in, moneyShape{AccountID: "acc", AmountMinor: int64(i), AssetCode: "IDR"})
	}
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(data) < 1<<20 {
		t.Fatalf("expected ≥1MB payload, got %d bytes", len(data))
	}
	var out []moneyShape
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(out) != len(in) || out[19999].AmountMinor != 19999 {
		t.Errorf("large payload mismatch: len %d", len(out))
	}
}

func TestGetTraversal(t *testing.T) {
	t.Parallel()

	doc := []byte(`{"entries":[{"amount_minor":1500,"asset":"USD"}],"meta":{"tenant":"t1"}}`)

	leaf, err := Get(doc, "entries", "0", "amount_minor")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(leaf) != "1500" {
		t.Errorf("unexpected leaf: %s", leaf)
	}

	if _, err := Get(doc, "entries", "3", "amount_minor"); err == nil {
		t.Error("expected out-of-range error, got nil")
	}
	if _, err := Get(doc, "meta", "missing"); err == nil {
		t.Error("expected missing-key error, got nil")
	}
	if _, err := Get(doc, "meta", "tenant", "deep"); err == nil {
		t.Error("expected descend-into-scalar error, got nil")
	}
	if _, err := Get([]byte(`{oops`), "a"); err == nil {
		t.Error("expected invalid-document error, got nil")
	}
}

func BenchmarkMarshal(b *testing.B) {
	in := moneyShape{AccountID: "acc_bench", AmountMinor: 123456, AssetCode: "USD"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(in); err != nil {
			b.Fatalf("Marshal: %v", err)
		}
	}
}
