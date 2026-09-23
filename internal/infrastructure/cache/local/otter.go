// Package local is the Otter L1 in-memory cache adapter (E08-T01):
// adaptive W-TinyLFU, per-key TTL, synchronous visibility. It implements the
// shared store.Store contract in []byte (the single representation shared
// with L2); typed access lives one layer up in hybrid views. Values are
// hints only and never authorize spending or establish uniqueness.
package local

import (
	"context"
	"fmt"
	"time"

	"github.com/maypok86/otter/v2"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
)

// DefaultMaximumWeight bounds L1 memory when wiring omits sizing (64 MiB of
// payload bytes; W-TinyLFU cost accounting makes the budget mean memory).
const DefaultMaximumWeight = 64 << 20

// DefaultTTL bounds L1 staleness when wiring omits a default.
const DefaultTTL = time.Minute

// OtterParams carries constructor dependencies (Parameter Object pattern).
// Zero values select the defaults above; per-key TTLs always come from Set.
type OtterParams struct {
	MaximumWeight uint64
	DefaultTTL    time.Duration
}

// OtterCache is the L1 handle over one *otter.Cache[string, []byte]
// instance. It implements store.Store. All methods are safe for concurrent
// use. Values are cloned on every boundary so a caller mutation can never
// corrupt cached state; nil payloads are legal negative-cache markers.
type OtterCache struct {
	cache      *otter.Cache[string, []byte]
	maxWeight  uint64
	defaultTTL time.Duration
}

// Compile-time seam conformance.
var _ store.Store = (*OtterCache)(nil)

// NewOtterCache builds the L1 cache with W-TinyLFU eviction keyed on byte
// cost and write-based expiry.
func NewOtterCache(params OtterParams) (*OtterCache, error) {
	maxWeight := params.MaximumWeight
	if maxWeight == 0 {
		maxWeight = DefaultMaximumWeight
	}

	defaultTTL := params.DefaultTTL
	if defaultTTL <= 0 {
		defaultTTL = DefaultTTL
	}

	cache, err := otter.New(&otter.Options[string, []byte]{
		MaximumWeight:    maxWeight,
		Weigher:          weighBytes,
		ExpiryCalculator: otter.ExpiryWriting[string, []byte](defaultTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("otter: new cache: %w", err)
	}

	return &OtterCache{cache: cache, maxWeight: maxWeight, defaultTTL: defaultTTL}, nil
}

// Get returns a copy of the cached value when present and unexpired, else
// store.ErrMiss. ctx is accepted for seam parity and ignored: a memory read
// cannot block.
func (c *OtterCache) Get(_ context.Context, key string) ([]byte, error) {
	if c == nil || c.cache == nil {
		return nil, fmt.Errorf("otter: cache is not initialized")
	}

	if key == "" {
		return nil, fmt.Errorf("otter: key is required")
	}

	value, ok := c.cache.GetIfPresent(key)
	if !ok {
		return nil, store.ErrMiss
	}

	return cloneBytes(value), nil
}

// Set stores a copy of value with a mandatory positive TTL. Set is
// synchronously visible to the next Get.
func (c *OtterCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.cache == nil {
		return fmt.Errorf("otter: cache is not initialized")
	}

	if key == "" {
		return fmt.Errorf("otter: key is required")
	}

	if ttl <= 0 {
		return fmt.Errorf("otter: ttl must be positive")
	}

	c.cache.Set(key, cloneBytes(value))
	c.cache.SetExpiresAfter(key, ttl)

	return nil
}

// Delete invalidates one key. Missing keys succeed.
func (c *OtterCache) Delete(_ context.Context, key string) error {
	if c == nil || c.cache == nil {
		return fmt.Errorf("otter: cache is not initialized")
	}

	if key == "" {
		return fmt.Errorf("otter: key is required")
	}

	c.cache.Invalidate(key)

	return nil
}

// Close stops Otter's background maintenance goroutines; it is safe to call
// twice. Wire it to the fx OnStop hook alongside the Valkey drain so process
// shutdown is graceful instead of leaking goroutines.
func (c *OtterCache) Close() error {
	if c == nil || c.cache == nil {
		return nil
	}

	c.cache.StopAllGoroutines()

	return nil
}

// Size reports the estimated entry count (observability only).
func (c *OtterCache) Size() int {
	if c == nil || c.cache == nil {
		return 0
	}

	return c.cache.EstimatedSize()
}

// MaximumWeight reports the wired memory budget (observability only).
func (c *OtterCache) MaximumWeight() uint64 {
	return c.maxWeight
}

// DefaultTTLValue reports the fallback TTL wired at construction.
func (c *OtterCache) DefaultTTLValue() time.Duration {
	if c == nil {
		return DefaultTTL
	}

	return c.defaultTTL
}

// weighBytes costs entries by payload length so the weight budget means
// memory. Lengths past the uint32 range saturate instead of wrapping.
func weighBytes(_ string, value []byte) uint32 {
	const maxUint32 = ^uint32(0)

	if uint64(len(value)) > uint64(maxUint32) {
		return maxUint32
	}

	return uint32(len(value)) //nolint:gosec // guarded by the range check above
}

// cloneBytes deep-copies payloads so caller mutations can never corrupt
// other readers or the stored representation.
func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}

	out := make([]byte, len(value))
	copy(out, value)

	return out
}
