package specification_test

import (
	"context"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestRefundWindowValid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	posted := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	inside := specification.RefundWindow{OriginalPostedAt: posted, Now: posted.Add(24 * time.Hour), Window: 30 * 24 * time.Hour}
	if res := specification.RefundWindowValid().Evaluate(ctx, inside); !res.Passed() {
		t.Fatalf("inside window must pass: %+v", res)
	}
	outside := specification.RefundWindow{OriginalPostedAt: posted, Now: posted.Add(31 * 24 * time.Hour), Window: 30 * 24 * time.Hour}
	if res := specification.RefundWindowValid().Evaluate(ctx, outside); res.Passed() || res.Violations[0].Code != "REFUND_WINDOW_EXPIRED" {
		t.Fatalf("outside window = %+v", res)
	}
	zero := specification.RefundWindow{}
	if res := specification.RefundWindowValid().Evaluate(ctx, zero); res.Passed() {
		t.Fatal("zero candidate must fail, not panic")
	}
}

func TestRefundAmountValid(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	exact := specification.RefundAmounts{OriginalMinor: 100, PreviousRefundsMinor: 40, RequestedMinor: 60, Asset: testUSD}
	if res := specification.RefundAmountValid().Evaluate(ctx, exact); !res.Passed() {
		t.Fatalf("exact remainder must pass: %+v", res)
	}
	over := specification.RefundAmounts{OriginalMinor: 100, PreviousRefundsMinor: 40, RequestedMinor: 61, Asset: testUSD}
	if res := specification.RefundAmountValid().Evaluate(ctx, over); res.Passed() || res.Violations[0].Code != "REFUND_EXCEEDS_ORIGINAL" {
		t.Fatalf("exceeding = %+v", res)
	}
	prior := specification.RefundAmounts{OriginalMinor: 100, PreviousRefundsMinor: 120, RequestedMinor: 1, Asset: testUSD}
	if res := specification.RefundAmountValid().Evaluate(ctx, prior); res.Passed() {
		t.Fatal("over-refunded history must fail")
	}
	neg := specification.RefundAmounts{OriginalMinor: 100, PreviousRefundsMinor: 0, RequestedMinor: -1, Asset: testUSD}
	if res := specification.RefundAmountValid().Evaluate(ctx, neg); res.Passed() {
		t.Fatal("negative request must fail closed")
	}
}
