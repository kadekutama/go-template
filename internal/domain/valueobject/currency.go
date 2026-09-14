package valueobject

import (
	"errors"
	"fmt"
	"time"
)

// AssetCode is a registry-backed currency/asset identifier (e.g. "USD").
// Codes are validated at registration; the sample lists in tests are examples,
// never the authority.
type AssetCode string

// Asset kinds carried by the registry.
const (
	AssetKindFiat      = "FIAT"
	AssetKindCrypto    = "CRYPTO"
	AssetKindCommodity = "COMMODITY"
)

// AssetInfo is one versioned registry entry: code, minor-unit exponent, kind,
// and activation window.
type AssetInfo struct {
	Code        AssetCode
	Exponent    int
	Kind        string
	ActiveFrom  time.Time
	ActiveUntil time.Time
}

// Registry maps asset codes to their metadata. It is caller-supplied and
// carries no global authority: unknown codes error at use sites.
type Registry struct {
	assets map[AssetCode]AssetInfo
}

// NewRegistry builds a registry from seed entries, rejecting invalid or
// duplicate codes.
func NewRegistry(infos ...AssetInfo) (Registry, error) {
	r := Registry{assets: make(map[AssetCode]AssetInfo, len(infos))}
	for _, info := range infos {
		if err := r.Register(info); err != nil {
			return Registry{}, err
		}
	}
	return r, nil
}

// Register adds one asset to the registry, rejecting empty codes, negative
// exponents, and duplicates.
func (r *Registry) Register(info AssetInfo) error {
	if info.Code == "" {
		return errors.New("currency: asset code is required")
	}
	if info.Exponent < 0 {
		return fmt.Errorf("currency: negative exponent for %q", info.Code)
	}
	if r.assets == nil {
		r.assets = make(map[AssetCode]AssetInfo)
	}
	if _, exists := r.assets[info.Code]; exists {
		return fmt.Errorf("currency: duplicate asset code %q", info.Code)
	}
	r.assets[info.Code] = info
	return nil
}

// Lookup returns the registry entry for code or an error when unknown.
func (r Registry) Lookup(code AssetCode) (AssetInfo, error) {
	info, ok := r.assets[code]
	if !ok {
		return AssetInfo{}, fmt.Errorf("currency: unknown asset code %q", code)
	}
	return info, nil
}
