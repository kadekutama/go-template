package event

import (
	"errors"
	"time"
)

const (
	fieldTenantID   = "tenant_id"
	fieldAccountID  = "account_id"
	fieldPostingID  = "posting_id"
	fieldPaymentID  = "payment_id"
	fieldPayoutID   = "payout_id"
	fieldTransferID = "transfer_id"
	fieldRefundID   = "refund_id"
	fieldLedgerID   = "ledger_id"
	fieldCode       = "code"
	fieldRunID      = "run_id"
	fieldBreakID    = "break_id"
	fieldPeriodID   = "period_id"
)

// TypedEvent is one catalog event: the immutable BaseEvent envelope plus a
// statically typed payload. EventType is stored at construction and never
// changes; new schema versions are new constructors with new type strings.
type TypedEvent[P any] struct {
	BaseEvent
	typed P
}

// Payload returns the typed event payload.
func (e TypedEvent[P]) Payload() any { return e.typed }

// Typed returns the statically typed payload without a type assertion.
func (e TypedEvent[P]) Typed() P { return e.typed }

// Compile-time proof that every TypedEvent satisfies DomainEvent.
var _ DomainEvent = TypedEvent[struct{}]{}

// newTyped validates envelope inputs through NewBaseEvent and attaches the
// typed payload. It returns an error on invalid input and never panics.
func newTyped[P any](eventType, eventID, aggregateID, aggregateType string, occurredAt time.Time, version, seq int64, payload P, meta EventMetadata) (TypedEvent[P], error) {
	base, err := NewBaseEvent(eventID, aggregateID, aggregateType, eventType, occurredAt, version, seq, nil, meta)
	if err != nil {
		return TypedEvent[P]{}, err
	}
	return TypedEvent[P]{BaseEvent: base, typed: payload}, nil
}

// requireIDs rejects empty identifiers shared by catalog constructors.
func requireIDs(fields map[string]string) error {
	for name, value := range fields {
		if value == "" {
			return errors.New("event: " + name + " is required")
		}
	}
	return nil
}
