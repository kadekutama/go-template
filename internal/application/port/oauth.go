package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// OAuthSession carries the login handshake state. State is caller-generated
// entropy verified on callback; PKCE verifiers never leave the edge.
type OAuthSession struct {
	Provider    string
	State       string
	RedirectURL string
}

// OAuthIdentity is the verified external identity linked to a tenant user.
// Provider user IDs are opaque strings; tokens are expended at exchange and
// never stored by callers.
type OAuthIdentity struct {
	TenantID     valueobject.TenantID
	Provider     string
	ProviderUser string
	Email        string
	VerifiedAt   time.Time
}

// OAuthProvider is the OAuth2/OIDC login boundary (E09 implements; no oauth2
// SDK types cross this contract). Exchanges are single-use: replayed codes
// fail. Sessions expire quickly and are bound to the requesting tenant.
type OAuthProvider interface {
	// AuthURL starts one login handshake for provider. Short-lived session.
	AuthURL(ctx context.Context, tenant valueobject.TenantID, provider, redirectURL string) (OAuthSession, error)
	// Exchange trades one authorization code + state for an identity. Single-use.
	Exchange(ctx context.Context, tenant valueobject.TenantID, provider, code, state string) (OAuthIdentity, error)
}
