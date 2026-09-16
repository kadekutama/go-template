package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// APIKey carries key metadata. The plaintext secret exists only in the Create
// response; adapters store hashes and never return plaintext again.
type APIKey struct {
	ID        string
	TenantID  valueobject.TenantID
	Name      string
	Prefix    string
	Scopes    []string
	ExpiresAt time.Time
	Revoked   bool
}

// APIKeySecret pairs metadata with its single plaintext reveal.
type APIKeySecret struct {
	Key       APIKey
	Plaintext string
}

// APIKeyStore mints, verifies, and revokes tenant API keys (E09 implements).
// Verification compares hashes in constant time and fails closed on unknown,
// expired, or revoked keys. Plaintext travels only the Create response.
type APIKeyStore interface {
	// Create mints one key with its single plaintext reveal. Strong write.
	Create(ctx context.Context, tenant valueobject.TenantID, name string, scopes []string, ttl time.Duration) (APIKeySecret, error)
	// Verify authenticates one plaintext key. Strong read; fails closed.
	Verify(ctx context.Context, plaintext string) (APIKey, error)
	// Revoke disables one key by ID. Strong write; idempotent.
	Revoke(ctx context.Context, tenant valueobject.TenantID, id string) error
}
