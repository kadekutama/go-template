package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// IdempotencyRecord reserves one idempotency key for a canonical request
// fingerprint. The fingerprint is computed by idempotency.Fingerprint (E06-T01)
// and stored opaquely here; this port never parses request bodies.
type IdempotencyRecord struct {
	// Key is the caller-supplied idempotency key, scoped by TenantID.
	Key string
	// Fingerprint is the canonical hash of the request.
	Fingerprint string
	// TenantID scopes the key to its tenant.
	TenantID valueobject.TenantID
}

// ReserveOutcome reports what a Reserve found. Replay is true only when this
// key already completed for the same fingerprint; Response is then the
// original stored response and the caller MUST NOT re-execute.
type ReserveOutcome struct {
	Replay   bool
	Response []byte
}

// IdempotencyStore is the durable idempotency-result boundary. Reads backing
// command decisions MUST be strong; a cache may only hint. Records live in
// the same atomic unit as the command's writes (see UnitOfWork). Keys scope
// by (TenantID, Key): one tenant's keys never collide with another's.
type IdempotencyStore interface {
	// Reserve leases Key for Fingerprint. An unknown key is leased for this
	// caller. A key completed for the same fingerprint replays its stored
	// response. A live (uncompleted) key reserved with the identical
	// fingerprint proceeds: reservation is reentrant for the same holder, so
	// pre-reserve probes and effect commits may share one key. A live key
	// held for a different fingerprint fails with an IDEMPOTENCY_CONFLICT
	// error and the caller MUST NOT execute. Strong read.
	Reserve(ctx context.Context, rec IdempotencyRecord) (ReserveOutcome, error)
	// Complete stores the command response for Key. Strong write, called
	// inside the command's UnitOfWork before commit.
	Complete(ctx context.Context, key string, response []byte) error
}
