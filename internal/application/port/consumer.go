package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Message is one consumed transport message. Handlers acknowledge by
// returning nil; any error redelivers per the group's policy until the DLQ
// (E14 owns groups, E08 owns transport). Event replay MUST NEVER re-run
// ledger commands: consumers apply read-side effects only.
type Message struct {
	// ID is the globally unique message identity for dedupe.
	ID string
	// TenantID scopes the message to its tenant.
	TenantID valueobject.TenantID
	// Subject is the transport subject (e.g. NATS subject).
	Subject string
	// Payload carries the event bytes, opaque to this contract.
	Payload []byte
	// Redelivered counts prior deliveries of this message.
	Redelivered int
	// PublishedAt is the transport publish time.
	PublishedAt time.Time
}

// MessageHandler processes one message to a read-side effect. Handlers are
// idempotent on Message.ID: redelivery MUST return the original effect.
type MessageHandler func(ctx context.Context, msg Message) error

// MessageConsumer is the consumer-group boundary (E08 transport, E14 groups).
// Run blocks while subscribed and returns only on ctx cancellation or fatal
// transport error; graceful shutdown drains in-flight handlers first.
type MessageConsumer interface {
	// Run subscribes with handler until ctx ends. At-least-once delivery.
	Run(ctx context.Context, tenant valueobject.TenantID, group, subject string, handler MessageHandler) error
}
