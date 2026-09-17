package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Secret is one retrieved secret value. Values MUST NOT be logged, embedded
// in errors, or cached beyond the stated TTL. Callers zero the slice when
// done holding it.
type Secret struct {
	// Name is the secret's canonical name.
	Name string
	// Value carries the secret bytes.
	Value []byte
	// ExpiresAt bounds configuration-driven caching, if any.
	ExpiresAt time.Time
}

// SecretStore is the secrets boundary (OpenBao in E09; ADR-016). Reads are strongly
// consistent; misses are caller errors, never silent empty values; rotation
// is transparent to callers behind the name.
type SecretStore interface {
	// Get returns one secret by tenant + name. Strong read.
	Get(ctx context.Context, tenant valueobject.TenantID, name string) (Secret, error)
}
