// Package store is the uniform cache-layer contract for this repository's
// caches (E08 review). Both backends — the in-process Otter L1 and the
// shared Valkey L2 — implement it with identical signatures, so either side
// can be swapped without touching callers. Backend-only capabilities stay on
// the concrete types (Otter: Size/Close; Valkey: SetNX/Eval/Ping/Close) and
// never enter this contract.
package store

import (
	"context"
	"errors"
	"time"
)

// NoExpiry is the remaining TTL reported for a key that exists without an
// expiration. Engines treat it as "bounded only by the configured populate
// limit" rather than as an expiring record.
const NoExpiry time.Duration = -1

// ErrMiss reports a cache miss. It is returned (never wrapped beyond
// recognition: use errors.Is) instead of a found flag so the contract reads
// like every other Go lookup (value, error).
var ErrMiss = errors.New("cache: miss")

// Store is the uniform Get/Set/Delete contract. Implementations MUST hand
// out copies so callers can never corrupt stored state. Nil payloads are
// legal (negative-cache markers); a non-positive TTL is an error on every
// backend. ctx cancels blocking backends; in-memory adapters accept and
// ignore it (a memory read cannot block) — documented, not hidden.
type Store interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

// ExpiringStore is implemented by backends that can report a record's
// remaining TTL (today: Valkey via PTTL). The engine uses it to bound L1
// repopulation so a copied entry never outlives its source record.
type ExpiringStore interface {
	Store
	TTL(ctx context.Context, key string) (time.Duration, error)
}
