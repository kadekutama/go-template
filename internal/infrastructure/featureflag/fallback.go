package featureflag

import (
	"context"
	"maps"
	"sync"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// fallbackEntry is one last-known flag value with its observation time.
type fallbackEntry struct {
	value      bool
	observedAt time.Time
}

// FallbackParams wires a Fallback (Parameter Object pattern).
type FallbackParams struct {
	// Clock supplies time for staleness. Required (application clock port).
	// Excluded from validator traversal like all seams; presence is checked
	// explicitly below.
	Clock appport.Clock `validate:"-"`
	// RefreshInterval bounds freshness. Required, positive.
	RefreshInterval time.Duration
	// Defaults supplies configured values per flag. Nil is valid. The map is
	// copied at construction; later caller mutations never affect lookups.
	Defaults map[string]bool
	// Cache optionally persists values (Valkey-backed in E08). Nil is valid.
	// Excluded from validator traversal like all seams.
	Cache appport.Cache `validate:"-"`
	// CacheTTL bounds L2 retention. Required positive: every environment
	// configures it explicitly; there is no default.
	CacheTTL time.Duration
	// Logger records cache-write failures. Nil-tolerant: a nil logger
	// disables failure logging. Flag keys and values never appear in lines.
	// Excluded from validator traversal like all seams.
	Logger log.Logger `validate:"-"`
}

// Fallback serves last-known values or configured defaults when Unleash is
// unreachable. Reads dominate (every flag evaluation); writes happen only on
// provider-confirmed observations, so a plain map with a documented RWMutex
// guards entries per repository convention (sync.Map is not used in
// adapters). A struct with a lock is never copied, so all methods use
// pointer receivers.
type Fallback struct {
	mu              sync.RWMutex
	entries         map[string]fallbackEntry
	defaults        map[string]bool
	clock           appport.Clock
	refreshInterval time.Duration
	cache           appport.Cache
	cacheTTL        time.Duration
	logger          log.Logger
}

// NewFallback builds the fallback store. Clock must be non-nil and
// RefreshInterval positive; Cache may be nil for in-memory-only operation;
// CacheTTL zero selects the one-hour default and negative is rejected.
func NewFallback(params FallbackParams) (*Fallback, error) {
	if params.Clock == nil {
		return nil, newConfigError("featureflag: fallback clock is required")
	}

	if params.RefreshInterval <= 0 {
		return nil, newConfigError("featureflag: fallback refresh interval must be positive")
	}

	if params.CacheTTL <= 0 {
		return nil, newConfigError("featureflag: fallback cache ttl must be positive")
	}

	cacheTTL := params.CacheTTL

	defaults := make(map[string]bool, len(params.Defaults))
	maps.Copy(defaults, params.Defaults)

	return &Fallback{
		entries:         make(map[string]fallbackEntry),
		defaults:        defaults,
		clock:           params.Clock,
		refreshInterval: params.RefreshInterval,
		cache:           params.Cache,
		cacheTTL:        cacheTTL,
		logger:          params.Logger,
	}, nil
}

// Observe records one provider-confirmed value with the current time. Cache
// persistence is best-effort: failures never block evaluation and never leak
// flag keys or values into errors.
func (f *Fallback) Observe(ctx context.Context, key string, value bool) {
	if f == nil || key == "" {
		return
	}

	now := f.clock.Now()

	f.mu.Lock()
	f.entries[key] = fallbackEntry{value: value, observedAt: now}
	ttl := f.cacheTTL
	f.mu.Unlock()

	if f.cache != nil {
		encoded := []byte{0}
		if value {
			encoded = []byte{1}
		}
		// Best-effort L2 write with the configured TTL. Failures never block
		// evaluation; they are logged (without flag keys or values) so a
		// degraded shared cache stays visible.
		if err := f.cache.Set(ctx, fallbackCacheKey(key), encoded, ttl); err != nil && f.logger != nil {
			f.logger.Warn(ctx, "featureflag: fallback cache write failed", "cause", err)
		}
	}
}

// Resolve returns the best in-memory fallback for key: last-known when
// present, otherwise the configured default (or def when unconfigured). The
// stale flag is true when last-known is past the refresh bound; defaults are
// never stale. It never returns an error: absence of data is a normal
// fallback. It never consults the shared cache; use ResolveWithContext when
// a context is available so cold starts can serve persisted last-known
// values.
func (f *Fallback) Resolve(key string, def bool) (bool, bool, bool) {
	if f == nil || key == "" {
		return def, false, false
	}

	now := f.clock.Now()

	f.mu.RLock()
	entry, ok := f.entries[key]
	f.mu.RUnlock()

	if ok {
		return entry.value, true, now.Sub(entry.observedAt) > f.refreshInterval
	}

	if configured, hasDefault := f.defaults[key]; hasDefault {
		return configured, false, false
	}

	return def, false, false
}

// ResolveWithContext resolves like Resolve and additionally consults the
// shared L2 cache on an in-memory miss, so cold starts and restarts serve
// persisted last-known values while Unleash is unreachable. L2 hits report
// stale because their original observation time is unknown. Cache failures
// and malformed entries fall through to defaults; absence of data is a
// normal fallback, never an error.
func (f *Fallback) ResolveWithContext(ctx context.Context, key string, def bool) (bool, bool, bool) {
	if f == nil || key == "" {
		return def, false, false
	}

	if err := ctx.Err(); err != nil {
		return f.Resolve(key, def)
	}

	value, fromLastKnown, stale := f.Resolve(key, def)
	if fromLastKnown || f.cache == nil {
		return value, fromLastKnown, stale
	}

	raw, err := f.cache.Get(ctx, fallbackCacheKey(key))
	if err != nil || len(raw) != 1 || (raw[0] != 0 && raw[0] != 1) {
		return value, false, false
	}

	return raw[0] == 1, true, true
}

// fallbackCacheKey scopes one flag value in the shared cache. Keys carry no
// tenant PII: flag names are operational identifiers, never user secrets.
func fallbackCacheKey(key string) string {
	return "flag:" + key
}
