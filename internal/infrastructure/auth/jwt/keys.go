package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sort"
	"sync/atomic"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// KeyEntry is one signing key version with its overlap expiry.
type KeyEntry struct {
	KID      string
	Private  *rsa.PrivateKey
	NotAfter time.Time
}

// KeyProvider supplies the active signing key and historic keys for
// verification during the rotation overlap window.
type KeyProvider interface {
	Active() (KeyEntry, error)
	ByID(kid string) (KeyEntry, error)
}

// keySnapshot is one immutable key set. Snapshots are never mutated after
// publication: rotation builds a new snapshot and swaps the pointer, so
// readers load lock-free with no mutex anywhere in the package.
type keySnapshot struct {
	active string
	keys   map[string]KeyEntry
}

// SnapshotProvider serves key versions from an atomic RCU snapshot.
// The snapshot is a cache of durable truth (OpenBao via SecretStore);
// a restart reloads it, so no key state is ever lost with the node.
// Rotation is a single-writer swap serialized by the caller (Redlock
// coordinator, E08-T02); concurrent swaps merge by kid, never tear reads.
type SnapshotProvider struct {
	current atomic.Pointer[keySnapshot]
	clock   appport.Clock
}

// SnapshotParams carries constructor dependencies.
type SnapshotParams struct {
	Clock  appport.Clock
	Keys   map[string]KeyEntry
	Active string
}

// NewSnapshotProvider publishes the initial snapshot. Keys must be non-empty
// and Active must name one of them.
func NewSnapshotProvider(params SnapshotParams) (*SnapshotProvider, error) {
	if len(params.Keys) == 0 || params.Active == "" {
		return nil, ErrKeyRequired
	}

	if _, ok := params.Keys[params.Active]; !ok {
		return nil, ErrNoActiveKey
	}

	provider := &SnapshotProvider{clock: params.Clock}
	provider.Swap(params.Keys, params.Active)

	return provider, nil
}

// Swap publishes a new snapshot, unioning with the current one so concurrent
// rotations never drop a version the other writer just published. The merge
// retries on contention: a loser re-reads the winner's snapshot and merges
// again instead of discarding it. Rotations are rare, so the loop always
// terminates in practice.
func (p *SnapshotProvider) Swap(keys map[string]KeyEntry, active string) {
	for {
		current := p.current.Load()

		var base map[string]KeyEntry
		if current != nil {
			base = current.keys
		}

		merged := make(map[string]KeyEntry, len(base)+len(keys))
		for kid, entry := range base {
			merged[kid] = entry
		}

		for kid, entry := range keys {
			entry.KID = kid
			merged[kid] = entry
		}

		if p.current.CompareAndSwap(current, &keySnapshot{active: active, keys: merged}) {
			return
		}
	}
}

// Active returns the current signing key without locking.
func (p *SnapshotProvider) Active() (KeyEntry, error) {
	snapshot := p.current.Load()
	if snapshot == nil {
		return KeyEntry{}, ErrNoActiveKey
	}

	entry, ok := snapshot.keys[snapshot.active]
	if !ok {
		return KeyEntry{}, ErrNoActiveKey
	}

	return entry, nil
}

// ByID returns one key version by kid without locking.
func (p *SnapshotProvider) ByID(kid string) (KeyEntry, error) {
	snapshot := p.current.Load()
	if snapshot == nil {
		return KeyEntry{}, ErrUnknownKeyID
	}

	entry, ok := snapshot.keys[kid]
	if !ok {
		return KeyEntry{}, ErrUnknownKeyID
	}

	return entry, nil
}

// ActiveSet returns the verifiable key set sorted by kid for JWKS rendering.
// It replaces direct lock/map access from other files.
func (p *SnapshotProvider) ActiveSet() []KeyEntry {
	snapshot := p.current.Load()
	if snapshot == nil {
		return nil
	}

	out := make([]KeyEntry, 0, len(snapshot.keys))
	for _, entry := range snapshot.keys {
		out = append(out, entry)
	}

	sort.Slice(out, func(i, j int) bool { return out[i].KID < out[j].KID })

	return out
}

// SnapshotFromPEM decodes one snapshot from PEM-encoded PKCS8/PKCS1 RSA
// private keys (the OpenBao storage shape). Overlap is derived per key at
// load time from now + overlap; callers pass the injected clock time.
func SnapshotFromPEM(pems map[string][]byte, active string, now time.Time, overlap time.Duration) (map[string]KeyEntry, error) {
	if len(pems) == 0 || active == "" {
		return nil, ErrKeyRequired
	}

	out := make(map[string]KeyEntry, len(pems))

	for kid, raw := range pems {
		key, err := parseRSAPrivateKey(raw)
		if err != nil {
			return nil, fmt.Errorf("jwt: key %s: %w", kid, err)
		}

		out[kid] = KeyEntry{KID: kid, Private: key, NotAfter: now.Add(overlap)}
	}

	if _, ok := out[active]; !ok {
		return nil, ErrNoActiveKey
	}

	return out, nil
}

// parseRSAPrivateKey decodes PEM PKCS8 or PKCS1 into an RSA private key
// using the standard library only.
func parseRSAPrivateKey(raw []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(raw)
	if block == nil {
		return nil, fmt.Errorf("%w: bad PEM", ErrInvalidClaims)
	}

	if parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if key, ok := parsed.(*rsa.PrivateKey); ok {
			return key, nil
		}

		return nil, fmt.Errorf("%w: not RSA", ErrInvalidClaims)
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: parse", ErrInvalidClaims)
	}

	return key, nil
}
