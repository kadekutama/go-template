// Package coalesce collapses concurrent identical cacheable reads into one
// provider call per process (E09.1-T04). It is a read-only optimization:
// it never authorizes spending, never establishes uniqueness, and cached
// entries remain hints per the hybrid contract. Money-moving operations
// cannot be expressed by its closed operation enum.
package coalesce

import (
	"context"
	"hash/fnv"
	"time"
	"unicode"

	"golang.org/x/sync/singleflight"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// Simple failures use the shared domain error (entity.Error); no local
// error type exists in this package.

// Typed error codes for coalescer misuse. All are returned before any
// provider call runs.
const (
	// CodeInvalidKey marks empty key fields or non-allow-listed kinds.
	CodeInvalidKey = "COALESCE_INVALID_KEY"
	// CodeInvalidKind marks rejected operation kinds.
	CodeInvalidKind = "COALESCE_INVALID_KIND"
	// CodeInvalidParams marks bad wiring or a nil work function.
	CodeInvalidParams = "COALESCE_INVALID_PARAMS"
)

// CacheableReadKind names one cacheable read shape. The set is closed by
// validation: unknown values — including any money-moving kind — are
// rejected before work starts, by type-level allow-listing rather than
// reviewer vigilance.
type CacheableReadKind string

const (
	// CacheableFXRate is the initial (and only) coalesced read: FX rate
	// cache-miss fetches (first consumer: E10-T02).
	CacheableFXRate CacheableReadKind = "fx_rate"
)

// Params wires a Coalescer (Parameter Object pattern). BaseContext
// is service-owned, credential-free, and MUST NOT be wrapped in
// context.WithoutCancel: graceful shutdown cancels active shared work.
// SharedTimeout bounds every shared call.
type Params struct {
	BaseContext   context.Context
	SharedTimeout time.Duration
}

// numShards spreads in-flight keys over independent singleflight groups.
// One group serializes all callers on a single mutex for map bookkeeping
// (held only across the lookup, never across provider work); under high RPS
// with many distinct keys that mutex becomes a hotspot, so keys hash to
// shards. Same key always lands on the same shard, preserving collapse.
const numShards = 32

// Coalescer collapses duplicate in-flight reads. Results and errors are
// shared only while the call is in flight; nothing is retained after
// completion. It is safe for concurrent use. Scope one Coalescer per
// provider (or provider family) in addition to sharding when a single
// process fans out to many vendors.
type Coalescer struct {
	shards  [numShards]singleflight.Group
	base    context.Context
	timeout time.Duration
}

// NewCoalescer validates the wiring. BaseContext must be non-nil (service
// shutdown propagates through it) and SharedTimeout must be positive.
func NewCoalescer(params Params) (*Coalescer, error) {
	if params.BaseContext == nil {
		return nil, entity.NewError(CodeInvalidParams, "coalesce: service base context is required")
	}

	if params.SharedTimeout <= 0 {
		return nil, entity.NewError(CodeInvalidParams, "coalesce: shared timeout must be positive")
	}

	return &Coalescer{base: params.BaseContext, timeout: params.SharedTimeout}, nil
}

// validKind reports whether the kind is an allow-listed cacheable read.
func validKind(kind CacheableReadKind) bool {
	return kind == CacheableFXRate
}

// Key scopes one in-flight read. Provider separates vendors, Scope
// separates tenant/authorization contexts (credentials or responses must
// never cross scopes), and RequestDigest is a non-PII request-identity
// digest. Identical digests under different providers or scopes form
// distinct in-flight keys.
type Key struct {
	Provider      string
	Scope         string
	RequestDigest string
	Kind          CacheableReadKind
}

// Validate rejects empty fields and non-allow-listed kinds before any work
// function runs. Control characters (including NUL) are rejected in every
// string component: the in-flight key joins components with NUL separators,
// so a NUL-bearing field could otherwise alias two distinct
// provider/scope/digest tuples into one shared result, violating scope
// separation. This mirrors the E05 key-segment rules.
func (k Key) Validate() error {
	if k.Provider == "" || k.Scope == "" || k.RequestDigest == "" {
		return entity.NewError(CodeInvalidKey, "coalesce: provider, scope, and request digest are required")
	}

	for _, field := range []string{k.Provider, k.Scope, k.RequestDigest} {
		for _, r := range field {
			if unicode.IsControl(r) {
				return entity.NewError(CodeInvalidKey, "coalesce: key fields must not contain control characters")
			}
		}
	}

	if !validKind(k.Kind) {
		return entity.Errorf(CodeInvalidKind, "coalesce: %q is not a coalescible cacheable read", string(k.Kind))
	}

	return nil
}

// Do executes fn once per distinct in-flight key and shares the outcome
// with concurrent identical callers. Each caller receives an independently
// mutable copy of response bytes. The caller context controls only that
// caller's wait: canceling one waiter never cancels shared work needed by
// others, while service shutdown (BaseContext) still cancels it. The shared
// callback must honor its context; Go cannot forcibly terminate a function
// that ignores cancellation.
func (c *Coalescer) Do(ctx context.Context, key Key, fn func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	if c == nil {
		return nil, entity.NewError(CodeInvalidParams, "coalesce: coalescer is not initialized")
	}

	if err := key.Validate(); err != nil {
		return nil, err
	}

	if fn == nil {
		return nil, entity.NewError(CodeInvalidParams, "coalesce: work function is required")
	}
	// The shared context is created inside the singleflight closure so it
	// lives exactly as long as the shared call: no waiter can cancel it by
	// returning early, while service shutdown and the hard timeout still
	// propagate through the service-owned base context.
	result := c.shard(key.inFlightKey()).DoChan(key.inFlightKey(), func() (any, error) {
		callCtx, callCancel := context.WithTimeout(c.base, c.timeout)
		defer callCancel()

		return fn(callCtx)
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case shared, ok := <-result:
		if !ok {
			return nil, context.Canceled
		}

		if shared.Err != nil {
			return nil, shared.Err
		}

		payload, ok := shared.Val.([]byte)
		if !ok {
			return nil, entity.NewError(CodeInvalidParams, "coalesce: work function returned a non-byte result")
		}

		return append([]byte(nil), payload...), nil
	}
}

// inFlightKey flattens the validated key for the singleflight group.
func (k Key) inFlightKey() string {
	return string(k.Kind) + "\x00" + k.Provider + "\x00" + k.Scope + "\x00" + k.RequestDigest
}

// shard maps an in-flight key to its group with FNV-1a.
func (c *Coalescer) shard(key string) *singleflight.Group {
	sum := fnv.New32a()
	_, _ = sum.Write([]byte(key))

	return &c.shards[sum.Sum32()&(numShards-1)]
}
