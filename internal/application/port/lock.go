package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Lock is one held distributed lease. Leases expire: holders refresh on long
// work and MUST treat expiry as loss of the lock, revalidating state before
// continuing. Locks coordinate workers and schedulers; they never gate ledger
// correctness, which rests on database serialization.
type Lock interface {
	// Release frees the lease. Idempotent: expired or missing leases succeed.
	Release(ctx context.Context) error
	// Refresh extends the lease by ttl. Fails when the lease is lost.
	Refresh(ctx context.Context, ttl time.Duration) error
}

// DistributedLock is the Redlock-style boundary (Valkey in E08). Acquire
// fails fast when the key is held; callers back off and retry. Lock keys are
// tenant-scoped by convention of the caller.
type DistributedLock interface {
	// Acquire takes the named lease for ttl or fails when held.
	Acquire(ctx context.Context, tenant valueobject.TenantID, key string, ttl time.Duration) (Lock, error)
}
