package valueobject

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
		return ErrPayoutPolicyTenantIsRequired
	}

	if p.AssetCode == "" {
		return ErrPayoutPolicyAssetCodeIsRequired
	}

	if p.MinimumMinor < 0 {
		return ErrPayoutPolicyMinimumMustBeNonNegative
	}

	if p.FirstPayoutHoldDays < 0 {
		return ErrPayoutPolicyFirstPayoutHoldMustBeNonNegative
	}

	if p.ReserveBPS < 0 || p.ReserveBPS > 10000 {
		return ErrPayoutPolicyReserveBpsMustBeWithin010000
	}

	if p.Version == "" {
		return ErrPayoutPolicyVersionIsRequired
	}

	return nil
}
