package service

import (
	"strings"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// prefixFXRate scopes FX rate keys in the shared cache and coalescer.
const prefixFXRate = "fx:"

// BuildFXRateKey returns the cache and single-flight identity
// fx:{base}:{quote}. The pair must be two distinct, key-safe codes; the key
// is an identity, never an authorization decision.
func BuildFXRateKey(base, quote valueobject.AssetCode) (string, error) {
	baseCode := valueobject.AssetCode(strings.TrimSpace(string(base)))
	quoteCode := valueobject.AssetCode(strings.TrimSpace(string(quote)))

	if _, err := valueobject.NewFxPair(baseCode, quoteCode); err != nil {
		return "", entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes")
	}

	for _, segment := range []string{string(baseCode), string(quoteCode)} {
		if err := validateKeySegment(segment); err != nil {
			return "", err
		}
	}

	return prefixFXRate + string(baseCode) + ":" + string(quoteCode), nil
}

// ParseFXRateKey inverts BuildFXRateKey.
func ParseFXRateKey(key string) (valueobject.AssetCode, valueobject.AssetCode, error) {
	rest, ok := strings.CutPrefix(key, prefixFXRate)
	if !ok {
		return "", "", entity.NewError("ISOLATION_KEY_INVALID", "fx rate key must start with 'fx:'")
	}

	parts := strings.Split(rest, ":")
	if len(parts) != exactParts2 {
		return "", "", entity.NewError("ISOLATION_KEY_INVALID", "fx rate key must have 2 segments")
	}

	for _, segment := range parts {
		if err := validateKeySegment(segment); err != nil {
			return "", "", err
		}
	}

	base := valueobject.AssetCode(strings.TrimSpace(parts[0]))

	quote := valueobject.AssetCode(strings.TrimSpace(parts[1]))
	if _, err := valueobject.NewFxPair(base, quote); err != nil {
		return "", "", entity.NewError("FX_PAIR_INVALID", "fx rate pair requires distinct base and quote codes")
	}

	return base, quote, nil
}
