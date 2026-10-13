package valueobject

import (
	"time"
)

// FxRate is a fixed-point conversion rate: quote = base × numerator /
// denominator. Floats are forbidden. RoundingPolicy is informational here
// (half-up at the conversion boundary); MarkToMarket vs realized-only is a
// versioned policy owned by callers.
type FxRate struct {
	ID             string
	Pair           FxPair
	Numerator      int64
	Denominator    int64
	Source         string
	QuotedAt       time.Time
	TTL            time.Duration
	RoundingPolicy string
}

// Validate checks the fixed-point shape. Errors follow the package
// convention (`fx: CODE detail`); callers map them to stable domain codes.
func (r FxRate) Validate() error {
	if r.ID == "" {
		return ErrFxFXRATEIDREQUIREDRateIDIsRequired
	}

	if r.Pair.Base == "" || r.Pair.Quote == "" {
		return ErrFxFXPAIRREQUIREDRateRequiresABaseQuotePair
	}

	if r.Numerator <= 0 || r.Denominator <= 0 {
		return ErrFxFXRATEINVALIDNumeratorAndDenominatorMustBePositive
	}

	if r.Source == "" {
		return ErrFxFXSOURCEREQUIREDRateSourceIsRequired
	}

	if r.QuotedAt.IsZero() {
		return ErrFxFXQUOTEDATREQUIREDQuoteTimeIsRequired
	}

	if r.TTL <= 0 {
		return ErrFxFXTTLREQUIREDRateTTLMustBePositive
	}

	return nil
}

// IsStale reports whether the rate is older than its TTL at time at.
func (r FxRate) IsStale(at time.Time) bool {
	return at.Sub(r.QuotedAt) > r.TTL
}
