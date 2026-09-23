package fakes

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/store"
)

// CacheStore is an in-memory store.Store + store.ExpiringStore test double:
// one fake serves the L1 role, the L2 role, or both, so cross-layer behavior
// (fallback order, populate bounding, delete order, fail-open) is provable
// without Docker. Production uses the Otter L1 and the Valkey L2.
type CacheStore struct {
	mu      sync.Mutex
	data    map[string][]byte
	expires map[string]time.Duration
	ops     []string
	shared  *Journal
	prefix  string
	failGet error
	failSet error
	failDel error
	failTTL error
}

// Journal captures ops across fakes in real time so cross-layer ordering
// (e.g. the L2 delete landing before the L1 delete) is provable; per-fake
// journals alone cannot show interleaving.
type Journal struct {
	mu  sync.Mutex
	ops []string
}

// Record appends one operation.
func (j *Journal) Record(op string) {
	j.mu.Lock()
	defer j.mu.Unlock()

	j.ops = append(j.ops, op)
}

// Snapshot returns a copy of the recorded operations in order.
func (j *Journal) Snapshot() []string {
	j.mu.Lock()
	defer j.mu.Unlock()

	out := make([]string, len(j.ops))
	copy(out, j.ops)

	return out
}

// NewCacheStore builds the double.
func NewCacheStore() *CacheStore {
	return &CacheStore{data: make(map[string][]byte), expires: make(map[string]time.Duration)}
}

// Compile-time seam conformance.
var (
	_ store.Store         = (*CacheStore)(nil)
	_ store.ExpiringStore = (*CacheStore)(nil)
)

// WithGetFailure arms Get to fail.
func (f *CacheStore) WithGetFailure(err error) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failGet = err

	return f
}

// WithSetFailure arms Set to fail.
func (f *CacheStore) WithSetFailure(err error) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failSet = err

	return f
}

// WithDeleteFailure arms Delete to fail.
func (f *CacheStore) WithDeleteFailure(err error) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failDel = err

	return f
}

// WithTTLFailure arms TTL to fail.
func (f *CacheStore) WithTTLFailure(err error) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failTTL = err

	return f
}

// Share wires a cross-layer journal that also records every op with the
// given layer prefix ("l1" or "l2").
func (f *CacheStore) Share(journal *Journal, layer string) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.shared = journal
	f.prefix = layer

	return f
}

// SetRemaining overrides the TTL the fake reports for key (test control for
// the populate bound: expiring records must not be copied).
func (f *CacheStore) SetRemaining(key string, remaining time.Duration) *CacheStore {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.expires[key] = remaining

	return f
}

// Get returns the stored copy or store.ErrMiss.
func (f *CacheStore) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.note("get")

	if f.failGet != nil {
		return nil, f.failGet
	}

	value, ok := f.data[key]
	if !ok {
		return nil, store.ErrMiss
	}

	out := make([]byte, len(value))
	copy(out, value)

	return out, nil
}

// Set stores a copy with the given TTL.
func (f *CacheStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	if key == "" {
		return fmt.Errorf("fakes: key is required")
	}

	if ttl <= 0 {
		return fmt.Errorf("fakes: ttl must be positive")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.note("set")

	if f.failSet != nil {
		return f.failSet
	}

	cp := make([]byte, len(value))
	copy(cp, value)
	f.data[key] = cp
	f.expires[key] = ttl

	return nil
}

// Delete invalidates one key. Missing keys succeed.
func (f *CacheStore) Delete(_ context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("fakes: key is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.note("delete")

	if f.failDel != nil {
		return f.failDel
	}

	delete(f.data, key)

	return nil
}

// TTL reports the remaining TTL the fake tracks, or store.ErrMiss for an
// absent key.
func (f *CacheStore) TTL(_ context.Context, key string) (time.Duration, error) {
	if key == "" {
		return 0, fmt.Errorf("fakes: key is required")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.note("ttl")

	if f.failTTL != nil {
		return 0, f.failTTL
	}

	remaining, ok := f.expires[key]
	if !ok {
		if _, known := f.data[key]; !known {
			return 0, store.ErrMiss
		}

		return 0, nil
	}

	return remaining, nil
}

// note appends to the fake's own journal and, when wired, to the shared
// cross-layer journal with the layer prefix.
func (f *CacheStore) note(op string) {
	f.ops = append(f.ops, op)

	if f.shared != nil {
		f.shared.Record(f.prefix + "." + op)
	}
}

// Journal returns a copy of the recorded operation names in order.
func (f *CacheStore) Journal() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]string, len(f.ops))
	copy(out, f.ops)

	return out
}

// AfterMarker returns the ops recorded after the first occurrence of marker,
// so assertions cover the post-setup delta instead of the whole journal.
func AfterMarker(ops []string, marker string) []string {
	for idx, op := range ops {
		if strings.TrimSpace(op) == marker {
			return ops[idx+1:]
		}
	}

	return ops
}

// Mark appends a marker entry to the journal (test control).
func (f *CacheStore) Mark(marker string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ops = append(f.ops, marker)
}
