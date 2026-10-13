package cronui

import (
	"context"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// VerifiedBearer carries the validated bearer facts the admin wrapper needs,
// in plain types (see SessionTokens for why this package avoids importing
// the interface layer).
type VerifiedBearer struct {
	Subject   string
	Scopes    []string
	ExpiresAt time.Time
}

// BearerTokens validates E09 bearer JWTs through the narrow token port. The
// E14 composition root injects the E09 issuer; rotation stays transparent
// behind key IDs inside that adapter's dependency.
type BearerTokens struct {
	issuer appport.TokenIssuer
}

// NewBearerTokens binds bearer verification to the token port.
func NewBearerTokens(issuer appport.TokenIssuer) (*BearerTokens, error) {
	if issuer == nil {
		return nil, ErrCronuiTokenIssuerIsRequired
	}

	return &BearerTokens{issuer: issuer}, nil
}

// VerifyBearer validates the Authorization header value ("Bearer <jwt>")
// and returns the subject, scopes, and expiry facts. The source token is
// never stored or logged.
func (b *BearerTokens) VerifyBearer(ctx context.Context, authorizationHeader string) (VerifiedBearer, error) {
	fail := func() (VerifiedBearer, error) {
		return VerifiedBearer{}, ErrCronuiBadBearerToken
	}
	if b == nil || b.issuer == nil {
		return fail()
	}

	token, found := strings.CutPrefix(authorizationHeader, "Bearer ")
	if !found || strings.TrimSpace(token) == "" {
		return fail()
	}

	claims, err := b.issuer.Verify(ctx, token)
	if err != nil {
		return fail()
	}

	return VerifiedBearer{Subject: claims.Subject, Scopes: claims.Scopes, ExpiresAt: claims.ExpiresAt}, nil
}
