package valueobject

import (
	"fmt"
)

// PayoutPolicy is the per-tenant/asset payout gate: minimums, first-payout
// hold, rolling reserve, instant eligibility. All values are versioned
// configuration supplied by the caller, never invented by the domain.
type PayoutPolicy struct {
	TenantID            TenantID
	AssetCode           AssetCode
	MinimumMinor        int64
	FirstPayoutHoldDays int
	ReserveBPS          int64
	InstantEligible     bool
	Version             string
}

// Validate checks the policy shape.
func (p PayoutPolicy) Validate() error {
	if p.TenantID.String() == "" {
		return fmt.Errorf("payout policy: tenant is required")
	}
	if p.AssetCode == "" {
		return fmt.Errorf("payout policy: asset code is required")
	}
	if p.MinimumMinor < 0 {
		return fmt.Errorf("payout policy: minimum must be non-negative")
	}
	if p.FirstPayoutHoldDays < 0 {
		return fmt.Errorf("payout policy: first-payout hold must be non-negative")
	}
	if p.ReserveBPS < 0 || p.ReserveBPS > 10000 {
		return fmt.Errorf("payout policy: reserve bps must be within [0,10000]")
	}
	if p.Version == "" {
		return fmt.Errorf("payout policy: version is required")
	}
	return nil
}
