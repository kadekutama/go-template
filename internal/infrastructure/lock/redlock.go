// Package lock is the Valkey Redlock coordinator (E08-T02): distributed
// leases for scheduler/reconciliation duplicate-work suppression behind
// port.DistributedLock. Locks coordinate workers; PostgreSQL locking and
// uniqueness remain the money-safety boundary. No Lua is used here: the
// only Lua script in the repo lives in E08-T07 ratelimit.lua.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// ErrLockHeld reports contention: the named lease is held by another holder.
var ErrLockHeld = errors.New("lock: held by another holder")

// ErrLockLost reports that a held lease no longer belongs to this holder
// (expired, evicted, or taken over). Holders MUST treat it as loss of
// exclusivity and revalidate state: a durable run/fencing key still decides
// what committed.
var ErrLockLost = errors.New("lock: lease was lost")

// Sentinel validation errors so constructor/argument failures are
// assertable with errors.Is/Equal without string matching.
var (
	ErrClientRequired     = errors.New("lock: valkey client is required")
	ErrNotInitialized     = errors.New("lock: not initialized")
	ErrLeaseNotInit       = errors.New("lock: lease is not initialized")
	ErrKeySegmentRequired = errors.New("lock: key segment is required")
	ErrKeySegmentTooLong  = errors.New("lock: key segment is too long")
	ErrKeySegmentColon    = errors.New("lock: key segment must not contain ':'")
	ErrTTLPositive        = errors.New("lock: ttl must be positive")
)

// DefaultTTL bounds one lease when callers pass ttl <= 0.
const DefaultTTL = 30 * time.Second

// maxKeySegment bounds lease key segments against abuse.
const maxKeySegment = 128

// ValkeyStore is the narrow Valkey seam for coordination (DIP): the
// production *valkey.ValkeyClient implements it and tests use an in-memory
// fake, so acquire/release/refresh paths are unit-testable without Docker.
type ValkeyStore interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	SetNX(ctx context.Context, key string, value []byte, ttl time.Duration) (bool, error)
}

// Compile-time seam conformance for the production client.
var _ ValkeyStore = (*valkey.ValkeyClient)(nil)

// RedlockParams carries constructor dependencies (Parameter Object pattern).
type RedlockParams struct {
	Client ValkeyStore
	Logger log.Logger
}

// Redlock implements port.DistributedLock over Valkey SET NX PX.
type Redlock struct {
	client ValkeyStore
	logger log.Logger
}

// Compile-time port conformance.
var _ appport.DistributedLock = (*Redlock)(nil)

// NewRedlock builds the coordinator; Client must be non-nil.
func NewRedlock(params RedlockParams) (*Redlock, error) {
	if params.Client == nil {
		return nil, ErrClientRequired
	}

	return &Redlock{client: params.Client, logger: params.Logger}, nil
}

// Acquire takes the named lease for ttl or fails fast with ErrLockHeld.
// Keys are tenant-scoped "lock:{tenant}:{key}".
func (r *Redlock) Acquire(ctx context.Context, tenant valueobject.TenantID, key string, ttl time.Duration) (appport.Lock, error) {
	if r == nil || r.client == nil {
		return nil, ErrNotInitialized
	}

	leaseKey, err := buildLockKey(tenant.String(), key)
	if err != nil {
		return nil, err
	}

	effective := ttl
	if effective <= 0 {
		effective = DefaultTTL
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("lock: acquire: %w", err)
	}

	token, err := newToken()
	if err != nil {
		return nil, err
	}

	won, err := r.client.SetNX(ctx, leaseKey, []byte(token), effective)
	if err != nil {
		return nil, err
	}

	if !won {
		return nil, ErrLockHeld
	}

	return &redlockLease{client: r.client, key: leaseKey, token: token, logger: r.logger}, nil
}

// redlockLease is one held lease. It is safe for single-goroutine use;
// holders refresh on long work and treat expiry as loss of the lock.
type redlockLease struct {
	client ValkeyStore
	key    string
	token  string
	logger log.Logger
}

// Compile-time port conformance.
var _ appport.Lock = (*redlockLease)(nil)

// Release frees the lease. Idempotent: expired, missing, or foreign leases
// succeed without deleting another holder's token.
func (l *redlockLease) Release(ctx context.Context) error {
	if l == nil || l.client == nil {
		return ErrLeaseNotInit
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("lock: release: %w", err)
	}

	current, err := l.client.Get(ctx, l.key)
	if err != nil {
		if errors.Is(err, valkey.ErrMiss) {
			return nil
		}

		return err
	}

	if string(current) != l.token {
		return nil
	}

	// Best-effort delete on token match. Coordination-only: durable writes
	// still validate fencing/run keys, so a racing expiry is safe.
	_ = l.client.Delete(ctx, l.key)

	return nil
}

// Refresh extends the lease by ttl. It fails with ErrLockLost when the lease
// is gone (expired, missing, or held by another token).
func (l *redlockLease) Refresh(ctx context.Context, ttl time.Duration) error {
	if l == nil || l.client == nil {
		return ErrLeaseNotInit
	}

	if ttl <= 0 {
		return ErrTTLPositive
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("lock: refresh: %w", err)
	}

	current, err := l.client.Get(ctx, l.key)
	if err != nil {
		if errors.Is(err, valkey.ErrMiss) {
			return fmt.Errorf("%w: %s expired or released", ErrLockLost, l.key)
		}

		return err
	}

	if string(current) != l.token {
		return fmt.Errorf("%w: %s taken over", ErrLockLost, l.key)
	}

	// Re-set with the same token to extend. A racing takeover between the
	// GET and SET is detected by the next Refresh/Get: only one token wins
	// the durable effect key downstream.
	if err := l.client.Set(ctx, l.key, []byte(l.token), ttl); err != nil {
		return err
	}

	return nil
}

func buildLockKey(tenant, key string) (string, error) {
	if err := validateSegment(tenant); err != nil {
		return "", err
	}

	if err := validateSegment(key); err != nil {
		return "", err
	}

	return "lock:" + strings.TrimSpace(tenant) + ":" + strings.TrimSpace(key), nil
}

func validateSegment(segment string) error {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "" {
		return ErrKeySegmentRequired
	}

	if len(trimmed) > maxKeySegment {
		return ErrKeySegmentTooLong
	}

	if strings.Contains(trimmed, ":") {
		return ErrKeySegmentColon
	}

	return nil
}

func newToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("lock: token: %w", err)
	}

	return hex.EncodeToString(raw[:]), nil
}
