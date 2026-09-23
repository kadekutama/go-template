package lock

import (
	"context"
	"errors"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/safe"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Sentinel validation errors so constructor/argument failures are
// assertable with errors.Is/Equal without string matching.
var (
	ErrLockerRequired = errors.New("lock: locker is required")
	ErrFnRequired     = errors.New("lock: fn is required")
)

// DefaultRefreshRatio derives the auto-renewal tick from the lease TTL.
const DefaultRefreshRatio = 3

// minRefreshInterval floors the renewal tick so short-lived leases in tests
// and local runs do not spin a tight timer.
const minRefreshInterval = time.Second

// GuardParams carries WithLock dependencies (Parameter Object pattern).
// RefreshInterval <= 0 derives leaseTTL/DefaultRefreshRatio (floored at one
// second); tests set it explicitly to observe auto-renewal cheaply.
type GuardParams struct {
	Locker          appport.DistributedLock                                            `validate:"-"`
	Logger          log.Logger                                                         `validate:"-"`
	RefreshInterval time.Duration                                                      `validate:"omitempty,gt=0"`
	OnContended     func(ctx context.Context, tenant valueobject.TenantID, key string) `validate:"-"`
	OnReleased      func(ctx context.Context, tenant valueobject.TenantID, key string) `validate:"-"`
}

// WithLock runs fn under the named lease, auto-renewing while it runs and
// releasing afterwards (even when the caller cancels or fn fails).
//
// Contention surfaces ErrLockHeld so callers back off and retry; only genuine
// contention triggers OnContended (validation and cancellation do not).
//
// If the lease is lost mid-run (expired, evicted, or taken over), the renewal
// loop cancels fn's context and its error (wrapping ErrLockLost) is returned:
// the protected work must stop and treat its effects as unconfirmed unless a
// durable run/fencing key proves otherwise.
func WithLock(ctx context.Context, params GuardParams, tenant valueobject.TenantID, key string, ttl time.Duration, fn func(ctx context.Context) error) error {
	if err := validate.Struct("lock", "guard params", params); err != nil {
		return err
	}

	if params.Locker == nil {
		return ErrLockerRequired
	}

	if fn == nil {
		return ErrFnRequired
	}

	lease, err := params.Locker.Acquire(ctx, tenant, key, ttl)
	if err != nil {
		return acquireError(ctx, params, err, tenant, key)
	}

	defer releaseLease(params, lease, tenant, key)

	effectiveTTL := ttl
	if effectiveTTL <= 0 {
		effectiveTTL = DefaultTTL
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	renewed := make(chan error, 1)
	group := safe.NewGroup(params.Logger, func(p safe.Panic) {
		renewed <- p
		cancel()
	})

	// Background renewal goroutine (SPEC §9.7): safe.Group contains a panic
	// in it (logged with stack, delivered to onPanic, cancelling fn) instead
	// of crashing the process, and Wait guarantees the lease release below
	// cannot race an in-flight Refresh.
	group.Go(runCtx, func(context.Context) {
		if renewErr := renew(runCtx, lease, effectiveTTL, refreshInterval(params.RefreshInterval, effectiveTTL)); renewErr != nil {
			renewed <- renewErr
			cancel()
		}
	})

	fnErr := fn(runCtx)

	cancel()
	group.Wait()

	var renewErr error

	select {
	case renewErr = <-renewed:
	default:
	}

	return guardError(fnErr, renewErr)
}

// acquireError surfaces contention through OnContended; validation, transport,
// and cancellation failures pass through untouched.
func acquireError(ctx context.Context, params GuardParams, err error, tenant valueobject.TenantID, key string) error {
	if errors.Is(err, ErrLockHeld) && params.OnContended != nil {
		params.OnContended(ctx, tenant, key)
	}

	return err
}

// releaseLease frees the lease on Background (not ctx): it must be freed even
// when the caller cancels or fn fails, otherwise the key stalls until TTL
// expiry. OnReleased observes the release for metrics/audits.
func releaseLease(params GuardParams, lease appport.Lock, tenant valueobject.TenantID, key string) {
	_ = lease.Release(context.Background())

	if params.OnReleased != nil {
		params.OnReleased(context.Background(), tenant, key)
	}
}

// guardError resolves the guard's return value. A lease lost mid-run is the
// root cause of fn seeing a cancelled context, so it takes precedence over
// that derived cancellation error (a renewal panic surfaces the same way as
// the contained safe.Panic). Any other fn error is the work's own failure
// and wins.
func guardError(fnErr, renewErr error) error {
	if fnErr == nil || (renewErr != nil && errors.Is(fnErr, context.Canceled)) {
		return renewErr
	}

	return fnErr
}

// renew extends the lease every interval until ctx ends. It returns the
// lease-loss error (which wraps ErrLockLost) so the guard can stop the work.
func renew(ctx context.Context, lease appport.Lock, ttl, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if ctx.Err() != nil {
				return nil
			}

			if err := lease.Refresh(ctx, ttl); err != nil {
				if errors.Is(err, context.Canceled) || ctx.Err() != nil {
					return nil
				}

				return err
			}
		}
	}
}

// refreshInterval derives the renewal tick: explicit value wins, otherwise
// leaseTTL/DefaultRefreshRatio floored at minRefreshInterval.
func refreshInterval(explicit, leaseTTL time.Duration) time.Duration {
	if explicit > 0 {
		return explicit
	}

	derived := leaseTTL / DefaultRefreshRatio
	if derived < minRefreshInterval {
		return minRefreshInterval
	}

	return derived
}
