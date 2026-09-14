package valueobject

import (
	"fmt"
)

// FxPair is a currency conversion pair quoted as base → quote.
type FxPair struct {
	Base  AssetCode
	Quote AssetCode
}

// NewFxPair validates a conversion pair: both codes required and distinct.
func NewFxPair(base, quote AssetCode) (FxPair, error) {
	if base == "" || quote == "" {
		return FxPair{}, fmt.Errorf("fx: pair requires base and quote codes")
	}
	if base == quote {
		return FxPair{}, fmt.Errorf("fx: pair base and quote must differ")
	}
	return FxPair{Base: base, Quote: quote}, nil
}
