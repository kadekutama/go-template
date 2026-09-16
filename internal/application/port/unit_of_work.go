package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/repository"
)

// UnitOfWork is the single atomic boundary for one command: postings,
// entries, holds, balance checkpoints, the durable idempotency result, and
// outbox facts commit together or not at all. Adapters own the transaction;
// callbacks never commit.
//
// The callback MUST be safe to run more than once: on retryable failures the
// adapter may re-execute it inside a fresh transaction. Callbacks therefore
// perform pure construction plus tx-scoped store calls only — no provider
// calls, no wall-clock reads (use Clock), no ID minting (use IDGenerator),
// and no observable side effects outside Tx.
type UnitOfWork interface {
	// Do runs fn inside one atomic transaction and commits on nil error.
	// A non-nil error rolls everything back. Commit/transport failures are
	// surfaced as errors without claiming success; callers resolve them via
	// idempotent replay, never blind retry.
	Do(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
}

// Tx is the atomic scope handed to one UnitOfWork callback. Stores are
// tx-bound views of the same contracts the domain ports define; the adapter
// binds them to the running transaction. Cursor is the ledger cursor of this
// transaction for subsequent strong reads.
type Tx interface {
	// Postings is the tx-scoped posting write/read path. Strong write/read.
	Postings() repository.PostingRepository
	// Holds is the tx-scoped hold write/read path. Strong write/read.
	Holds() repository.HoldRepository
	// Idempotency is the tx-scoped durable idempotency result store.
	Idempotency() IdempotencyStore
	// Outbox stages facts committed atomically with the writes.
	Outbox() EventOutbox
	// Cursor returns the ledger cursor of this transaction.
	Cursor() string
}
