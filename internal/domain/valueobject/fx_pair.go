package valueobject

// FxPair is a currency conversion pair quoted as base → quote.
type FxPair struct {
	Base  AssetCode
	Quote AssetCode
}

// NewFxPair validates a conversion pair: both codes required and distinct.
func NewFxPair(base, quote AssetCode) (FxPair, error) {
	if base == "" || quote == "" {
		return FxPair{}, ErrFxPairRequiresBaseAndQuoteCodes
	}

	if base == quote {
		return FxPair{}, ErrFxPairBaseAndQuoteMustDiffer
	}

	return FxPair{Base: base, Quote: quote}, nil
}
