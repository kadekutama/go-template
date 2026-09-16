package port

import (
	"context"
	"time"
)

// Cache is the read-model cache boundary (hybrid Ristretto + Valkey in E08).
// Cached figures back displays and hints ONLY: spend decisions, eligibility,
// and idempotency MUST read the strong projection. Invalidation is explicit
// per key; TTLs bound staleness and are always set.
type Cache interface {
	// Get returns the cached value or a miss error. Read-through hint only.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores value with a mandatory TTL. Best-effort write.
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	// Delete invalidates one key. Best-effort write.
	Delete(ctx context.Context, key string) error
}
