package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TokenClaims is the verified identity carried by an access token. Scopes
// are opaque strings interpreted by the Authorizer adapter; tokens carry no
// permissions beyond scope names and no PII beyond the subject.
type TokenClaims struct {
	Subject   string
	TenantID  valueobject.TenantID
	Scopes    []string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// TokenIssuer mints and verifies RS256 access tokens (E09 implements;
// rotation is transparent behind key IDs). Verification is strongly
// consistent against current keys; revoked tokens fail closed. Tokens never
// substitute for command-level Authorizer checks.
type TokenIssuer interface {
	// Issue mints one token for claims with the adapter's active key.
	Issue(ctx context.Context, claims TokenClaims) (string, error)
	// Verify validates signature, expiry, and revocation. Strong read.
	Verify(ctx context.Context, token string) (TokenClaims, error)
}
