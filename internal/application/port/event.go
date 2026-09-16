package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// OutboxFact is one durable, atomically-committed notification fact. Payload
// is opaque bytes owned by the emitting bounded context; serialization and
// delivery live with the relay (E08), not with this contract.
type OutboxFact struct {
	// TenantID scopes the fact to its tenant and is always required.
	TenantID valueobject.TenantID
	// LedgerID scopes accounting facts inside the tenant.
	LedgerID valueobject.LedgerID
	// EventType is the versioned type (e.g. "transfer.completed.v1").
	EventType string
	// AggregateID identifies the emitting aggregate.
	AggregateID string
	// Payload is the immutable event payload, opaque to this contract.
	Payload []byte
	// OccurredAt is the UTC emission time.
	OccurredAt time.Time
}

// EventOutbox stages facts inside the command's UnitOfWork so notification
// facts commit atomically with the writes they describe. It never delivers.
type EventOutbox interface {
	// Append stages facts in the running transaction. Strong write.
	Append(ctx context.Context, facts ...OutboxFact) error
}

// EventPublisher relays committed outbox facts to the transport (E08). It
// runs outside any command transaction and MUST be safe to invoke more than
// once per fact: consumers are idempotent on the fact identity.
type EventPublisher interface {
	// Publish relays already-committed facts. At-least-once delivery.
	Publish(ctx context.Context, facts ...OutboxFact) error
}
