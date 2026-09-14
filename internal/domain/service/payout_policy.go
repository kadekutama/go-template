package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// EligibilityInput is the payout gate decision request. AvailableMinor is the
// policy-derived spendable amount from the STRONG projection at Cursor;
// cached/replica figures must never be passed here.
type EligibilityInput struct {
	Policy              valueobject.PayoutPolicy
	AvailableMinor      int64
	RequestedMinor      int64
	AssetCode           valueobject.AssetCode
	TenantAgeDays       int
	DestinationVerified bool
	Method              valueobject.PayoutMethod
	Cursor              string
}

// Eligibility reasons for blocked payouts.
const (
	ReasonBelowMinimum          = "BELOW_MINIMUM"
	ReasonFirstPayoutHold       = "FIRST_PAYOUT_HOLD"
	ReasonReserveShortfall      = "RESERVE_SHORTFALL"
	ReasonNegativeAvailable     = "NEGATIVE_AVAILABLE"
	ReasonDestinationUnverified = "DESTINATION_UNVERIFIED"
	ReasonInstantIneligible     = "INSTANT_INELIGIBLE"
)

// EligibilityResult is the payout gate decision: input errors surface as
// error (caller bug / bad scope); policy blocks surface as a non-eligible
// result with a stable reason. No string parsing is needed downstream.
type EligibilityResult struct {
	Eligible bool
	Reason   string
}

// BlockedError maps a blocked result to the wire error. Nil results (or
// eligible ones) yield nil.
func (r EligibilityResult) BlockedError() *entity.Error {
	if r.Eligible {
		return nil
	}
	return entity.NewError("PAYOUT_BLOCKED", "payout blocked ("+r.Reason+")")
}

// EvaluateEligibility applies minimum, first-payout hold, rolling reserve,
// instant eligibility, destination verification, and available balance.
func EvaluateEligibility(in EligibilityInput) (EligibilityResult, error) {
	if err := validateEligibilityInput(in); err != nil {
		return EligibilityResult{}, err
	}
	if reason, blocked := checkEligibilityGating(in); blocked {
		return EligibilityResult{Reason: reason}, nil
	}
	if reason, blocked := checkEligibilityBalance(in); blocked {
		return EligibilityResult{Reason: reason}, nil
	}
	return EligibilityResult{Eligible: true}, nil
}

func validateEligibilityInput(in EligibilityInput) error {
	if err := in.Policy.Validate(); err != nil {
		return entity.NewError("PAYOUT_POLICY_INVALID", "payout policy is invalid")
	}
	if in.Cursor == "" {
		return entity.NewError("CURSOR_REQUIRED", "eligibility requires the strong-projection cursor")
	}
	if in.AssetCode != in.Policy.AssetCode {
		return entity.NewError("CURRENCY_MISMATCH", "payout asset differs from policy asset")
	}
	if in.RequestedMinor <= 0 {
		return entity.NewError("INVALID_PAYOUT_AMOUNT", "payout amount must be positive")
	}
	return nil
}

func checkEligibilityGating(in EligibilityInput) (string, bool) {
	if !in.DestinationVerified {
		return ReasonDestinationUnverified, true
	}
	if in.RequestedMinor < in.Policy.MinimumMinor {
		return ReasonBelowMinimum, true
	}
	if in.TenantAgeDays < in.Policy.FirstPayoutHoldDays {
		return ReasonFirstPayoutHold, true
	}
	if in.Method == valueobject.PayoutRTP || in.Method == valueobject.PayoutFedNow {
		if !in.Policy.InstantEligible {
			return ReasonInstantIneligible, true
		}
	}
	return "", false
}

func checkEligibilityBalance(in EligibilityInput) (string, bool) {
	if in.AvailableMinor < 0 {
		return ReasonNegativeAvailable, true
	}
	reserve := in.AvailableMinor * in.Policy.ReserveBPS / 10000
	spendable := in.AvailableMinor - reserve
	if spendable < in.RequestedMinor {
		return ReasonReserveShortfall, true
	}
	return "", false
}
