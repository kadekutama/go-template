package jwt

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Verify validates signature, expiry, issuer, audience, and revocation.
// It is a strong read against current keys and fails closed.
func (s *Issuer) Verify(ctx context.Context, token string) (appport.TokenClaims, error) {
	if s == nil || s.keys == nil || s.clock == nil {
		return appport.TokenClaims{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.TokenClaims{}, fmt.Errorf("jwt: verify: %w", err)
	}

	if token == "" {
		return appport.TokenClaims{}, fmt.Errorf("%w: empty token", ErrInvalidToken)
	}

	parsed, err := jwt.NewParser(jwt.WithTimeFunc(s.now)).ParseWithClaims(token, &tokenClaims{}, s.keyFunc)
	if err != nil {
		return appport.TokenClaims{}, classifyParseError(err)
	}

	claims, ok := parsed.Claims.(*tokenClaims)
	if !ok || !parsed.Valid {
		return appport.TokenClaims{}, ErrInvalidToken
	}

	return s.toPortClaims(claims)
}

// keyFunc resolves the verification key by kid header.
func (s *Issuer) keyFunc(token *jwt.Token) (any, error) {
	if token.Method != jwt.SigningMethodRS256 {
		return nil, fmt.Errorf("%w: unexpected method %v", ErrInvalidSignature, token.Header["alg"])
	}

	kid, _ := token.Header["kid"].(string)
	if kid == "" {
		return nil, ErrUnknownKeyID
	}

	entry, err := s.keys.ByID(kid)
	if err != nil {
		return nil, err
	}

	now := s.now()
	if !entry.NotAfter.IsZero() && now.After(entry.NotAfter.Add(s.rotationGrace)) {
		return nil, fmt.Errorf("%w: key %s past grace", ErrUnknownKeyID, kid)
	}

	return &entry.Private.PublicKey, nil
}

// toPortClaims maps verified JWT claims onto the port shape with issuer,
// audience, and expiry checks.
func (s *Issuer) toPortClaims(claims *tokenClaims) (appport.TokenClaims, error) {
	if claims.Issuer != s.issuer {
		return appport.TokenClaims{}, fmt.Errorf("%w: issuer mismatch", ErrInvalidClaims)
	}

	if !audienceMatches(claims.Audience, s.audience) {
		return appport.TokenClaims{}, fmt.Errorf("%w: audience mismatch", ErrInvalidClaims)
	}

	if claims.Subject == "" || claims.TenantID == "" {
		return appport.TokenClaims{}, fmt.Errorf("%w: subject/tenant required", ErrInvalidClaims)
	}

	now := s.now()
	if claims.ExpiresAt == nil || now.After(claims.ExpiresAt.Time) {
		return appport.TokenClaims{}, ErrExpired
	}

	issued := now
	if claims.IssuedAt != nil {
		issued = claims.IssuedAt.Time
	}

	return appport.TokenClaims{
		Subject:   claims.Subject,
		TenantID:  valueobject.TenantID(claims.TenantID),
		Scopes:    append([]string(nil), claims.Scopes...),
		IssuedAt:  issued,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

// audienceMatches reports whether want appears in the token audience list.
func audienceMatches(audience jwt.ClaimStrings, want string) bool {
	return slices.Contains(audience, want)
}

// classifyParseError maps jwt library errors onto sentinel errors.
func classifyParseError(err error) error {
	if err == nil {
		return nil
	}

	lowered := strings.ToLower(err.Error())

	switch {
	case strings.Contains(lowered, "expired"):
		return ErrExpired
	case strings.Contains(lowered, "signature"):
		return fmt.Errorf("%w: %s", ErrInvalidSignature, err.Error())
	case strings.Contains(lowered, "unknown key") || strings.Contains(lowered, "past grace"):
		return fmt.Errorf("%w: %s", ErrUnknownKeyID, err.Error())
	default:
		return fmt.Errorf("%w: %s", ErrInvalidToken, err.Error())
	}
}
