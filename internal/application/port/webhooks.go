package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// WebhookMessage is one outbound tenant webhook delivery. Payloads match the
// api-contracts §10 shapes byte-for-byte; signing uses the endpoint secret
// held by the adapter, never exposed here.
type WebhookMessage struct {
	TenantID   valueobject.TenantID
	EndpointID string
	EventType  string
	Payload    []byte
	OccurredAt time.Time
}

// WebhookDispatcher is the tenant-webhook delivery boundary (E08 implements
// with retries, backoff, and per-endpoint circuit breaking). Dispatch is
// at-least-once per message: receivers dedupe on the event ID inside the
// payload. Dispatcher state never gates ledger writes.
type WebhookDispatcher interface {
	// Dispatch enqueues one webhook message for delivery. Idempotent per event ID.
	Dispatch(ctx context.Context, message WebhookMessage) error
}
