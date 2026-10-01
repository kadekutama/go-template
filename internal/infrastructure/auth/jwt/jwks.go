package jwt

import (
	"crypto/rsa"
	"encoding/base64"
	"math/big"
)

// JWK is one RSA public key in JWKS format.
type JWK struct {
	Kty string `json:"kty"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKS is the published key set for verifiers.
type JWKS struct {
	Keys []JWK `json:"keys"`
}

// keySetSource is the snapshot surface JWKS renders from. SnapshotProvider
// implements it; any future KeyProvider that can enumerate versions can too.
type keySetSource interface {
	ActiveSet() []KeyEntry
}

// JWKS publishes the active plus overlap keys for verifiers.
func (s *Issuer) JWKS() JWKS {
	if s == nil || s.keys == nil {
		return JWKS{}
	}

	source, ok := s.keys.(keySetSource)
	if !ok {
		if entry, err := s.keys.Active(); err == nil && entry.Private != nil {
			return JWKS{Keys: []JWK{encodeJWK(entry.KID, &entry.Private.PublicKey)}}
		}

		return JWKS{}
	}

	now := s.now()
	out := make([]JWK, 0)

	for _, entry := range source.ActiveSet() {
		if entry.Private == nil {
			continue
		}

		if !entry.NotAfter.IsZero() && now.After(entry.NotAfter.Add(s.rotationGrace)) {
			continue
		}

		out = append(out, encodeJWK(entry.KID, &entry.Private.PublicKey))
	}

	return JWKS{Keys: out}
}

// encodeJWK maps one RSA public key onto JWK fields.
func encodeJWK(kid string, pub *rsa.PublicKey) JWK {
	return JWK{
		Kty: "RSA",
		Use: "sig",
		Kid: kid,
		N:   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}
}
