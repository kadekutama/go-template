// Package hybrid is the L1+L2 read-model cache (E08-T01): one shared engine
// over two byte-stores plus zero-cost typed views.
//
// Shape: Cache (non-generic) owns the L1 bytes and the shared L2 client and
// satisfies port.Cache directly; Typed[V] binds a typed view to it with the
// single engine-wide codec. Services import only this package:
//
//	engine := hybrid.New(hybrid.Params{L1: l1, L2: l2})   // L1/L2 built once, injected
//	var cache port.Cache = engine                        // port consumers
//	balances := hybrid.Typed[Balance](engine)            // pure view, no resources
//	b, err := balances.Get(ctx, key)                     // typed, no hand-encoding
//
// An L1 hit on a typed view decodes once (µs at our payload sizes); the
// alternative — one Otter instance per type — fragments budgets and warm-up
// for no measurable gain here. Cached figures back displays and hints only:
// spend decisions, eligibility, and idempotency read the strong projection.
// Invalidation runs only after the authoritative DB commit. Keys MUST come
// from the E05-T03 isolation builders; one key belongs to exactly one view.
package hybrid

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Sentinel errors for the cache contract. Validation failures return these
// directly (assertable with errors.Is/Equal); the ctx and degraded-L2 paths
// wrap them without losing the sentinel.
var (
	// ErrCacheMiss reports a miss in both layers. A degraded L2 also
	// surfaces as (and wraps) ErrCacheMiss: reads fail open to the strong
	// projection rather than failing the request, per the E08-T01 failure
	// semantics.
	ErrCacheMiss = errors.New("cache: miss")

	// ErrNotInitialized reports use of a cache built without required seams.
	ErrNotInitialized = errors.New("cache: not initialized")

	// ErrKeyRequired reports an empty cache key.
	ErrKeyRequired = errors.New("cache: key is required")

	// ErrTTLPositive reports a non-positive TTL; caching always expires.
	ErrTTLPositive = errors.New("cache: ttl must be positive")

	// ErrL1Required reports a constructor call without the L1 seam.
	ErrL1Required = errors.New("cache: L1 is required")

	// ErrUnsupportedValueType reports a value whose Go type can never be
	// cached (function, channel, unsafe pointer, complex). Nil values of
	// nil-able types (slice/map/pointer/interface) stay legal: they are the
	// negative-cache marker.
	ErrUnsupportedValueType = errors.New("cache: unsupported value type")
)

// Per-data-class staleness defaults (ADR-015 + E08-T01 §3). They are
// unexported on purpose: runtime values come from configuration through
// TTLSet/DefaultTTLs, so no caller hardcodes them.
const (
	defaultBalanceTTL         = time.Minute
	defaultBalanceCursorTTL   = 5 * time.Minute
	defaultConfigTTL          = 5 * time.Minute
	defaultConfigLongTTL      = 30 * time.Minute
	defaultFXTTL              = time.Hour
	defaultIdempotencyHintTTL = 24 * time.Hour
	defaultRateLimitTTL       = time.Minute
	defaultL1PopulateTTL      = time.Minute
)

// TTLSet carries the per-data-class staleness bounds so wiring can override
// them per environment; DefaultTTLs returns the contract defaults. Callers
// pass the chosen TTL to Set; L1Populate bounds how long an L2 hit may live
// in L1.
type TTLSet struct {
	Balance         time.Duration
	BalanceCursor   time.Duration
	Config          time.Duration
	ConfigLong      time.Duration
	FX              time.Duration
	IdempotencyHint time.Duration
	RateLimit       time.Duration
	L1Populate      time.Duration
}

// DefaultTTLs returns the contract TTL defaults.
func DefaultTTLs() TTLSet {
	return TTLSet{
		Balance:         defaultBalanceTTL,
		BalanceCursor:   defaultBalanceCursorTTL,
		Config:          defaultConfigTTL,
		ConfigLong:      defaultConfigLongTTL,
		FX:              defaultFXTTL,
		IdempotencyHint: defaultIdempotencyHintTTL,
		RateLimit:       defaultRateLimitTTL,
		L1Populate:      defaultL1PopulateTTL,
	}
}

// WithDefaults fills every zero field from DefaultTTLs (zero means "unset")
// and rejects negative bounds: partial configuration overlays the contract
// defaults, while an explicit negative value is a configuration error rather
// than something to silently replace.
func (t TTLSet) WithDefaults() (TTLSet, error) {
	defaults := DefaultTTLs()

	fields := []struct {
		name  string
		value *time.Duration
	}{
		{"balance", &t.Balance},
		{"balance_cursor", &t.BalanceCursor},
		{"config", &t.Config},
		{"config_long", &t.ConfigLong},
		{"fx", &t.FX},
		{"idempotency_hint", &t.IdempotencyHint},
		{"rate_limit", &t.RateLimit},
		{"l1_populate", &t.L1Populate},
	}

	fallbacks := []time.Duration{
		defaults.Balance,
		defaults.BalanceCursor,
		defaults.Config,
		defaults.ConfigLong,
		defaults.FX,
		defaults.IdempotencyHint,
		defaults.RateLimit,
		defaults.L1Populate,
	}

	for idx, field := range fields {
		switch {
		case *field.value < 0:
			return TTLSet{}, fmt.Errorf("cache: ttl %s must not be negative", field.name)
		case *field.value == 0:
			*field.value = fallbacks[idx]
		}
	}

	return t, nil
}

// Validate rejects negative TTLs after defaults are applied; every cached
// entry must expire, so a zero or negative bound is a configuration error.
func (t TTLSet) Validate() error {
	bounds := []struct {
		name string
		ttl  time.Duration
	}{
		{"balance", t.Balance},
		{"balance_cursor", t.BalanceCursor},
		{"config", t.Config},
		{"config_long", t.ConfigLong},
		{"fx", t.FX},
		{"idempotency_hint", t.IdempotencyHint},
		{"rate_limit", t.RateLimit},
		{"l1_populate", t.L1Populate},
	}

	for _, bound := range bounds {
		if bound.ttl <= 0 {
			return fmt.Errorf("cache: ttl %s must be positive", bound.name)
		}
	}

	return nil
}

// Codec translates typed values to the byte representation shared with L1/L2.
// It is intentionally non-generic (Encode takes any, Decode takes a
// destination pointer) so ONE instance selected in New serves every typed
// view; the default is JSON via the repository jsonparser seam.
type Codec interface {
	Encode(value any) ([]byte, error)
	Decode(data []byte, into any) error
}

// JSONCodec encodes structured values with the repository jsonparser seam.
type JSONCodec struct{}

// Encode marshals the value.
func (JSONCodec) Encode(value any) ([]byte, error) {
	return jsonparser.Marshal(value)
}

// Decode unmarshals the stored bytes into the pointed-to value.
func (JSONCodec) Decode(data []byte, into any) error {
	return jsonparser.Unmarshal(data, into)
}

// Params carries constructor dependencies (Parameter Object pattern). The
// composition root builds (or selects) the L1/L2 instances and injects them;
// this package never constructs adapters. L2 may be nil for L1-only
// operation by passing an untyped nil. Codec nil selects the JSON default.
type Params struct {
	L1            store.Store
	L2            store.ExpiringStore
	Codec         Codec
	L1PopulateTTL time.Duration
	Logger        log.Logger
}

// Cache is the shared cache engine: one L1 plus one L2 for every typed view
// built from it. It satisfies port.Cache directly ([]byte API; nil value =
// negative marker). Build once at the composition root with New.
type Cache struct {
	l1            store.Store
	l2            store.ExpiringStore
	sharedCodec   Codec
	l1PopulateTTL time.Duration
	logger        log.Logger
}

// Engine is the cache engine contract callers program against — services
// receive this interface, never the concrete *Cache (SOLID/DIP: depend on
// the abstraction). It embeds the application port so port consumers keep
// working unchanged. The codec method is unexported, which seals the
// interface: only *Cache in this package can implement it, so no external
// implementation can silently break the typed views' translation.
type Engine interface {
	appport.Cache

	// codec returns the engine-wide codec translating every typed view.
	codec() Codec
}

// Compile-time conformance: the engine satisfies its own contract and the
// application port.
var (
	_ Engine        = (*Cache)(nil)
	_ appport.Cache = (*Cache)(nil)
)

// New builds the engine from injected, already-shared instances. Codec nil
// selects the JSON default; L1PopulateTTL <= 0 selects the contract default.
func New(params Params) (*Cache, error) {
	if params.L1 == nil {
		return nil, ErrL1Required
	}

	codec := params.Codec
	if codec == nil {
		codec = JSONCodec{}
	}

	l1PopulateTTL := params.L1PopulateTTL
	if l1PopulateTTL <= 0 {
		l1PopulateTTL = defaultL1PopulateTTL
	}

	return &Cache{
		l1:            params.L1,
		l2:            params.L2,
		sharedCodec:   codec,
		l1PopulateTTL: l1PopulateTTL,
		logger:        params.Logger,
	}, nil
}

// Get returns the bytes from L1, else reads L2 once and populates L1 bounded
// by the record's remaining TTL. A degraded L2 fails open as ErrCacheMiss
// per the E08-T01 failure semantics.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	if c == nil || c.l1 == nil {
		return nil, ErrNotInitialized
	}

	if key == "" {
		return nil, ErrKeyRequired
	}

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("cache: get: %w", err)
	}

	if value, err := c.l1.Get(ctx, key); err != nil {
		if !errors.Is(err, store.ErrMiss) {
			c.warn(ctx, "cache.l1.degraded", key, err)
		}
	} else {
		return value, nil
	}

	if c.l2 == nil {
		return nil, ErrCacheMiss
	}

	data, err := c.l2.Get(ctx, key)
	if err != nil {
		if errors.Is(err, store.ErrMiss) {
			return nil, ErrCacheMiss
		}

		c.warn(ctx, "cache.l2.degraded", key, err)

		return nil, fmt.Errorf("%w: l2 unavailable: %w", ErrCacheMiss, err)
	}

	c.populateL1(ctx, key, data)

	return data, nil
}

// Set stores bytes with a mandatory positive TTL in both layers. Nil values
// are legal: negative caching stores "no such record" so repeated lookups do
// not stampede the database. Callers MUST distinguish a negative entry from
// a miss by the returned error (nil error = entry exists) rather than by the
// value.
func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.l1 == nil {
		return ErrNotInitialized
	}

	if key == "" {
		return ErrKeyRequired
	}

	if ttl <= 0 {
		return ErrTTLPositive
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cache: set: %w", err)
	}

	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	if c.l2 == nil {
		return nil
	}

	if err := c.l2.Set(ctx, key, value, ttl); err != nil {
		return err
	}

	return nil
}

// Delete invalidates one key in both layers. Missing keys succeed.
//
// L2 is deleted first: it is the remote, failable write. If it fails, L1 is
// left untouched and the error surfaces so the caller can retry; clearing L1
// first would let the next Get repopulate L1 from the still-stale L2 record.
// Callers MUST invoke Delete only after the authoritative DB commit; a
// concurrent Get may still repopulate from L2 between the two deletes, so
// writers MUST NOT treat Delete as a linearizable fence (post-commit
// invalidation bounds staleness, it does not eliminate the race).
func (c *Cache) Delete(ctx context.Context, key string) error {
	if c == nil || c.l1 == nil {
		return ErrNotInitialized
	}

	if key == "" {
		return ErrKeyRequired
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("cache: delete: %w", err)
	}

	if c.l2 != nil {
		if err := c.l2.Delete(ctx, key); err != nil {
			return err
		}
	}

	if err := c.l1.Delete(ctx, key); err != nil {
		return err
	}

	return nil
}

// populateL1 copies an L2 hit into L1 without ever extending its lifetime
// beyond the L2 record: the copy TTL is min(configured bound, remaining),
// and an already-expiring record (or a TTL read failure) skips the copy.
func (c *Cache) populateL1(ctx context.Context, key string, value []byte) {
	if c.l2 == nil {
		return
	}

	remaining, err := c.l2.TTL(ctx, key)
	if err != nil {
		if !errors.Is(err, store.ErrMiss) {
			c.warn(ctx, "cache.l2.ttl.degraded", key, err)
		}

		return
	}

	ttl := c.l1PopulateTTL

	switch {
	case remaining == store.NoExpiry:
	case remaining <= 0:
		return
	case remaining < ttl:
		ttl = remaining
	}

	if err := c.l1.Set(ctx, key, value, ttl); err != nil {
		c.warn(ctx, "cache.l1.populate.failed", key, err)
	}
}

func (c *Cache) warn(ctx context.Context, msg, key string, cause error) {
	if c.logger == nil {
		return
	}

	c.logger.Warn(ctx, msg, "key", key, "cause", cause)
}

// codec returns the engine-wide codec for typed views.
func (c *Cache) codec() Codec {
	if c == nil {
		return JSONCodec{}
	}

	return c.sharedCodec
}

// TypedCache is one typed projection over the shared engine: values cross
// the API as V while storage stays bytes. It holds the Engine interface, not
// the concrete *Cache, so views stay decoupled from the implementation. It
// holds no resources of its own — bind it once per projection with Typed
// and share the engine everywhere.
type TypedCache[V any] struct {
	engine Engine
}

// Typed binds a typed view to the shared engine. It performs no I/O and
// allocates no backend resources, so one engine serves any number of views;
// the engine-wide codec (chosen once in New) translates every view.
func Typed[V any](engine Engine) *TypedCache[V] {
	return &TypedCache[V]{engine: engine}
}

// Get returns the typed value, decoding once through the engine codec. A
// degraded L2 fails open as ErrCacheMiss per the E08-T01 failure semantics.
func (t *TypedCache[V]) Get(ctx context.Context, key string) (V, error) {
	var zero V

	if t == nil || t.engine == nil {
		return zero, ErrNotInitialized
	}

	data, err := t.engine.Get(ctx, key)
	if err != nil {
		return zero, err
	}

	var out V
	if err := t.engine.codec().Decode(data, &out); err != nil {
		return zero, fmt.Errorf("cache: decode %s: %w", key, err)
	}

	return out, nil
}

// Set stores a typed value with a mandatory positive TTL, encoding once
// through the engine codec. Nil values of nil-able V stay legal (negative
// caching); values of non-cacheable Go kinds (func/chan/unsafe
// pointer/complex) are rejected up front.
func (t *TypedCache[V]) Set(ctx context.Context, key string, value V, ttl time.Duration) error {
	if t == nil || t.engine == nil {
		return ErrNotInitialized
	}

	if err := validateValue(value); err != nil {
		return err
	}

	data, err := t.engine.codec().Encode(value)
	if err != nil {
		return fmt.Errorf("cache: encode %s: %w", key, err)
	}

	return t.engine.Set(ctx, key, data, ttl)
}

// Delete invalidates one key in both layers. Missing keys succeed.
func (t *TypedCache[V]) Delete(ctx context.Context, key string) error {
	if t == nil || t.engine == nil {
		return ErrNotInitialized
	}

	return t.engine.Delete(ctx, key)
}

// validateValue rejects Go kinds that can never be cached while allowing nil
// values of nil-able kinds (negative caching). A nil interface reports
// reflect.Invalid and is allowed; an interface holding a func is caught by
// its dynamic kind.
func validateValue[V any](value V) error {
	reflected := reflect.ValueOf(value)

	switch reflected.Kind() {
	case reflect.Func, reflect.Chan, reflect.UnsafePointer, reflect.Complex64, reflect.Complex128:
		return fmt.Errorf("%w: %s", ErrUnsupportedValueType, reflected.Kind())
	default:
		return nil
	}
}
