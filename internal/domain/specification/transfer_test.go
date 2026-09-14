package specification_test

import (
	"context"
	"math"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func mustMoney(minor int64, asset valueobject.AssetCode) valueobject.Money {
	return valueobject.MustMoney(minor, asset)
}

func TestTransferAmountPositive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	good := specification.TransferCandidate{From: testAccount1, To: testAccount2, Amount: mustMoney(50, testUSD)}
	if res := specification.TransferAmountPositive().Evaluate(ctx, good); !res.Passed() {
		t.Fatalf("positive must pass: %+v", res)
	}
	zero := specification.TransferCandidate{From: testAccount1, To: testAccount2, Amount: mustMoney(0, testUSD)}
	if res := specification.TransferAmountPositive().Evaluate(ctx, zero); res.Passed() || res.Violations[0].Code != "INVALID_TRANSFER_AMOUNT" {
		t.Fatalf("zero = %+v", res)
	}
}

func TestSufficientFunds(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spec := specification.SufficientFunds(specification.BalanceSnapshot{Available: mustMoney(100, testUSD)})
	if res := spec.Evaluate(ctx, mustMoney(100, testUSD)); !res.Passed() {
		t.Fatalf("exact available must pass: %+v", res)
	}
	res := spec.Evaluate(ctx, mustMoney(101, testUSD))
	if res.Passed() || res.Violations[0].Code != "INSUFFICIENT_FUNDS" {
		t.Fatalf("overspend = %+v", res)
	}
	if res.Violations[0].Details["available"] != "100" || res.Violations[0].Details["required"] != "101" {
		t.Fatalf("details = %+v", res.Violations[0])
	}
	res = spec.Evaluate(ctx, mustMoney(50, testEUR))
	if res.Passed() || res.Violations[0].Code != "CURRENCY_MISMATCH" {
		t.Fatalf("cross-asset = %+v", res)
	}
}

func TestCaptureAmountValid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ok := specification.CaptureRequest{AuthorizedMinor: 100, CapturedTotalMinor: 60, CaptureMinor: 40, Asset: testUSD}
	if res := specification.CaptureAmountValid().Evaluate(ctx, ok); !res.Passed() {
		t.Fatalf("exact capture must pass: %+v", res)
	}
	over := specification.CaptureRequest{AuthorizedMinor: 100, CapturedTotalMinor: 60, CaptureMinor: 41, Asset: testUSD}
	if res := specification.CaptureAmountValid().Evaluate(ctx, over); res.Passed() || res.Violations[0].Code != "CAPTURE_EXCEEDS_AUTHORIZED" {
		t.Fatalf("over-capture = %+v", res)
	}
	neg := specification.CaptureRequest{AuthorizedMinor: 100, CapturedTotalMinor: 0, CaptureMinor: -1, Asset: testUSD}
	if res := specification.CaptureAmountValid().Evaluate(ctx, neg); res.Passed() {
		t.Fatal("negative capture must fail closed")
	}
}

func TestAllocationExact(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.AllocationExact().Evaluate(ctx, specification.Allocation{Source: 100, Shares: []int64{34, 33, 33}}); !res.Passed() {
		t.Fatal("exact shares must pass")
	}
	if res := specification.AllocationExact().Evaluate(ctx, specification.Allocation{Source: 100, Shares: []int64{34, 33, 32}}); res.Passed() || res.Violations[0].Code != "UNBALANCED_TRANSACTION" {
		t.Fatalf("off-by-one = %+v", res)
	}
	if res := specification.AllocationExact().Evaluate(ctx, specification.Allocation{Source: 0}); res.Passed() {
		t.Fatal("empty shares must fail")
	}
	overflow := specification.Allocation{Source: 1, Shares: []int64{math.MaxInt64, math.MaxInt64}}
	if res := specification.AllocationExact().Evaluate(ctx, overflow); res.Passed() {
		t.Fatal("overflowing sum must fail, not wrap to pass")
	}
}
