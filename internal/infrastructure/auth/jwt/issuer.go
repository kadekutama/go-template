package jwt

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// JWTParams carries constructor dependencies (Parameter Object pattern).
type JWTParams struct {
	Issuer        string        `validate:"required,max=256"`
	Audience      string        `validate:"required,max=256"`
	AccessTTL     time.Duration `validate:"required,gt=0"`
	RefreshTTL    time.Duration `validate:"required,gt=0"`
	RotationGrace time.Duration `validate:"gte=0"`
	Keys          KeyProvider
	Clock         appport.Clock
}

// Issuer mints and verifies RS256 access tokens. Verification consults the
// key provider so rotation is transparent behind key IDs. Refresh rotation
// lives in ReceiptStore (durable PostgreSQL receipts); JWTParams.RefreshTTL
// carries the paired TTL so wiring passes one value to both.
type Issuer struct {
	issuer        string
	audience      string
	accessTTL     time.Duration
	refreshTTL    time.Duration
	rotationGrace time.Duration
	keys          KeyProvider
	clock         appport.Clock
}

// Compile-time port conformance.
var _ appport.TokenIssuer = (*Issuer)(nil)

// NewIssuer builds the JWT adapter; Keys and Clock must be non-nil.
func NewIssuer(params JWTParams) (*Issuer, error) {
	if params.Keys == nil {
		return nil, ErrKeyRequired
	}

	if params.Clock == nil {
		return nil, ErrClockRequired
	}

	if err := validate.Struct("jwt", "params", params); err != nil {
		return nil, err
	}

	return &Issuer{
		issuer:        params.Issuer,
		audience:      params.Audience,
		accessTTL:     params.AccessTTL,
		refreshTTL:    params.RefreshTTL,
		rotationGrace: params.RotationGrace,
		keys:          params.Keys,
		clock:         params.Clock,
	}, nil
}

// now returns the current UTC time through the injected clock.
func (s *Issuer) now() time.Time {
	return s.clock.Now().UTC()
}

// tokenClaims are the JWT private claims for access tokens.
type tokenClaims struct {
	jwt.RegisteredClaims
	TenantID string   `json:"tenant_id"`
	Scopes   []string `json:"scopes"`
}

// Issue mints one RS256 access token for claims with the active key.
func (s *Issuer) Issue(ctx context.Context, claims appport.TokenClaims) (string, error) {
	if s == nil || s.keys == nil || s.clock == nil {
		return "", ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("jwt: issue: %w", err)
	}

	if claims.Subject == "" || claims.TenantID == "" {
		return "", fmt.Errorf("%w: subject and tenant are required", ErrInvalidClaims)
	}

	entry, err := s.keys.Active()
	if err != nil {
		return "", err
	}

	now := s.now()

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, tokenClaims{
		Issuer:    s.issuer,
		Audience:  jwt.ClaimStrings{s.audience},
		Subject:   claims.Subject,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		TenantID:  string(claims.TenantID),
		Scopes:    append([]string(nil), claims.Scopes...),
	})
	token.Header["kid"] = entry.KID

	signed, err := token.SignedString(entry.Private)
	if err != nil {
		return "", fmt.Errorf("jwt: sign: %w", err)
	}

	return signed, nil
}
