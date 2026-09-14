package specification

import (
	"context"
	"strconv"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RefundWindow is a refund timeliness check: original posting time, decision
// time, and the policy window.
type RefundWindow struct {
	OriginalPostedAt time.Time
	Now              time.Time
	Window           time.Duration
}

// RefundAmounts is a refund remainder check in one asset.
type RefundAmounts struct {
	OriginalMinor        int64
	PreviousRefundsMinor int64
	RequestedMinor       int64
	Asset                valueobject.AssetCode
}

// RefundWindowValid passes when now - original.posted_at stays within the
// policy window.
func RefundWindowValid() Specification[RefundWindow] {
	return evalFunc[RefundWindow](func(_ context.Context, w RefundWindow) SpecResult {
		if w.Window < 0 || w.OriginalPostedAt.IsZero() || w.Now.IsZero() {
			return SpecResult{Violations: []Violation{{
				Code: "REFUND_WINDOW_EXPIRED", Message: "refund window inputs invalid",
			}}}
		}
		if w.Now.Sub(w.OriginalPostedAt) > w.Window {
			return SpecResult{Violations: []Violation{{
				Code: "REFUND_WINDOW_EXPIRED", Message: "refund outside policy window",
				Details: map[string]string{
					"window": w.Window.String(),
				},
			}}}
		}
		return SpecResult{}
	})
}

// RefundAmountValid passes when the requested refund fits the unrefunded
// remainder (original - previous refunds) with overflow-safe comparison.
// Negative inputs fail closed.
func RefundAmountValid() Specification[RefundAmounts] {
	return evalFunc[RefundAmounts](func(_ context.Context, r RefundAmounts) SpecResult {
		fail := func(detail string) SpecResult {
			return SpecResult{Violations: []Violation{{
				Code: "REFUND_EXCEEDS_ORIGINAL", Message: "refund exceeds unrefunded remainder",
				Details: map[string]string{
					keyAsset:           string(r.Asset),
					"original":         strconv.FormatInt(r.OriginalMinor, 10),
					"previous_refunds": strconv.FormatInt(r.PreviousRefundsMinor, 10),
					"requested":        strconv.FormatInt(r.RequestedMinor, 10),
					keyDetail:          detail,
				},
			}}}
		}
		if r.OriginalMinor < 0 || r.PreviousRefundsMinor < 0 || r.RequestedMinor < 0 {
			return fail("negative input")
		}
		if r.PreviousRefundsMinor > r.OriginalMinor {
			return fail("previous refunds exceed original")
		}
		remaining := r.OriginalMinor - r.PreviousRefundsMinor
		if r.RequestedMinor > remaining {
			return fail("requested exceeds remainder")
		}
		return SpecResult{}
	})
}
