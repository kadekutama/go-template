package specification

import (
	"context"
	"math"
	"strconv"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TransferCandidate is a transfer spend request.
type TransferCandidate struct {
	From   valueobject.AccountID
	To     valueobject.AccountID
	Amount valueobject.Money
}

// BalanceSnapshot is the spendable balance a spend decision is taken against.
// It is a caller-supplied strong-read projection; specs never read stores.
type BalanceSnapshot struct {
	Available valueobject.Money
}

// CaptureRequest is an incremental capture against an authorization.
type CaptureRequest struct {
	AuthorizedMinor    int64
	CapturedTotalMinor int64
	CaptureMinor       int64
	Asset              valueobject.AssetCode
}

// Allocation is a split whose shares must sum exactly to the source.
type Allocation struct {
	Source int64
	Shares []int64
}

// TransferAmountPositive passes for strictly positive transfer amounts.
func TransferAmountPositive() Specification[TransferCandidate] {
	return NewFuncSpec[TransferCandidate]("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive",
		func(_ context.Context, c TransferCandidate) bool { return c.Amount.IsPositive() })
}

// SufficientFunds passes when the snapshot available balance covers the spend
// amount in the same asset. Cross-asset spend fails CURRENCY_MISMATCH; both
// figures are reported on failure.
func SufficientFunds(snapshot BalanceSnapshot) Specification[valueobject.Money] {
	available := snapshot.Available
	return evalFunc[valueobject.Money](func(_ context.Context, spend valueobject.Money) SpecResult {
		fail := func(code, message string) SpecResult {
			return SpecResult{Violations: []Violation{{
				Code: code, Message: message,
				Details: map[string]string{
					keyAsset:    string(available.Asset()),
					"available": strconv.FormatInt(available.AmountMinor(), 10),
					"required":  strconv.FormatInt(spend.AmountMinor(), 10),
				},
			}}}
		}
		if spend.Asset() != available.Asset() {
			return fail("CURRENCY_MISMATCH", "spend asset differs from balance asset")
		}
		cmp, err := available.Compare(spend)
		if err != nil {
			return fail("CURRENCY_MISMATCH", "spend asset differs from balance asset")
		}
		if cmp < 0 {
			return fail("INSUFFICIENT_FUNDS", "available balance below spend amount")
		}
		return SpecResult{}
	})
}

// CaptureAmountValid passes when captured_total + capture stays within the
// authorized amount. Negative inputs fail closed.
func CaptureAmountValid() Specification[CaptureRequest] {
	return evalFunc[CaptureRequest](func(_ context.Context, r CaptureRequest) SpecResult {
		fail := func(detail string) SpecResult {
			return SpecResult{Violations: []Violation{{
				Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount",
				Details: map[string]string{
					keyAsset:         string(r.Asset),
					"authorized":     strconv.FormatInt(r.AuthorizedMinor, 10),
					"captured_total": strconv.FormatInt(r.CapturedTotalMinor, 10),
					"capture":        strconv.FormatInt(r.CaptureMinor, 10),
					keyDetail:        detail,
				},
			}}}
		}
		if r.AuthorizedMinor < 0 || r.CapturedTotalMinor < 0 || r.CaptureMinor < 0 {
			return fail("negative input")
		}
		if r.CaptureMinor > r.AuthorizedMinor-r.CapturedTotalMinor {
			return fail("total exceeds authorized")
		}
		return SpecResult{}
	})
}

// AllocationExact passes iff posted shares sum exactly to the source amount
// (money-flow §2.13 construction rule alongside AllocateLargestRemainder).
func AllocationExact() Specification[Allocation] {
	return NewFuncSpec[Allocation]("UNBALANCED_TRANSACTION", "shares do not sum to source",
		func(_ context.Context, a Allocation) bool {
			var sum int64
			for _, s := range a.Shares {
				if (s > 0 && sum > math.MaxInt64-s) || (s < 0 && sum < math.MinInt64-s) {
					return false
				}
				sum += s
			}
			return len(a.Shares) > 0 && sum == a.Source
		})
}
