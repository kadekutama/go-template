// Package event provides the immutable domain event envelope consumed by
// every aggregate in E02-T03–T05 and cataloged in E02-T06.
package event

import (
	"errors"
	"maps"
	"time"
)

// EventMetadata carries tenant scoping and correlation context for a domain event.
type EventMetadata struct {
	// TenantID scopes the event to its tenant and is always required.
	TenantID string `json:"tenant_id"`
	// LedgerID scopes accounting facts inside the tenant.
	LedgerID string `json:"ledger_id,omitempty"`
	// CausationID links the event to the command that caused it.
	CausationID string `json:"causation_id,omitempty"`
	// CorrelationID groups related events across aggregates.
	CorrelationID string `json:"correlation_id,omitempty"`
	// UserID identifies the actor that initiated the action.
	UserID string `json:"user_id,omitempty"`
	// TraceID carries distributed-tracing correlation.
	TraceID string `json:"trace_id,omitempty"`
	// Custom holds additional string context. It must not carry secrets,
	// idempotency keys, or unrestricted PII.
	Custom map[string]string `json:"custom,omitempty"`
}

// DomainEvent is the narrow consumer contract for domain events. Ordering is
// defined by AggregateVersion and Sequence, never by wall-clock timestamps.
type DomainEvent interface {
	// EventID returns the globally unique event identifier.
	EventID() string
	// AggregateID returns the identifier of the emitting aggregate.
	AggregateID() string
	// AggregateType returns the kind of aggregate (Account, Posting, Hold, ...).
	AggregateType() string
	// EventType returns the versioned event type (e.g. "account.created.v1").
	EventType() string
	// OccurredAt returns when the event happened, normalized to UTC.
	OccurredAt() time.Time
	// AggregateVersion returns the emitting aggregate version at emission time.
	AggregateVersion() int64
	// Sequence returns the per-aggregate ordering sequence.
	Sequence() int64
	// Payload returns the event-specific immutable data.
	Payload() any
	// Metadata returns a copy of the correlation metadata.
	Metadata() EventMetadata
}

// BaseEvent is the immutable envelope implementation. All state is unexported
// and exposed through value-receiver accessors; there are no mutators.
type BaseEvent struct {
	eventID          string
	aggregateID      string
	aggregateType    string
	eventType        string
	occurredAt       time.Time
	aggregateVersion int64
	sequence         int64
	payload          any
	metadata         EventMetadata
}

// Compile-time proof that BaseEvent satisfies DomainEvent.
var _ DomainEvent = BaseEvent{}

// NewBaseEvent validates its inputs and returns an immutable BaseEvent.
// IDs, types, tenant scope, and timestamps are caller-supplied (application
// layer via the kernel IDGenerator/Clock); the domain never generates them.
// The custom metadata map is defensively copied. It returns an error on
// invalid input and never panics.
func NewBaseEvent(eventID, aggregateID, aggregateType, eventType string, occurredAt time.Time, aggregateVersion, sequence int64, payload any, meta EventMetadata) (BaseEvent, error) {
	if eventID == "" {
		return BaseEvent{}, errors.New("event: event_id is required")
	}
	if aggregateID == "" {
		return BaseEvent{}, errors.New("event: aggregate_id is required")
	}
	if aggregateType == "" {
		return BaseEvent{}, errors.New("event: aggregate_type is required")
	}
	if eventType == "" {
		return BaseEvent{}, errors.New("event: event_type is required")
	}
	if meta.TenantID == "" {
		return BaseEvent{}, errors.New("event: metadata tenant_id is required")
	}
	if occurredAt.IsZero() {
		return BaseEvent{}, errors.New("event: occurred_at is required")
	}
	if aggregateVersion < 0 {
		return BaseEvent{}, errors.New("event: aggregate_version must not be negative")
	}
	if sequence < 0 {
		return BaseEvent{}, errors.New("event: sequence must not be negative")
	}
	meta.Custom = maps.Clone(meta.Custom)
	return BaseEvent{
		eventID:          eventID,
		aggregateID:      aggregateID,
		aggregateType:    aggregateType,
		eventType:        eventType,
		occurredAt:       occurredAt.UTC(),
		aggregateVersion: aggregateVersion,
		sequence:         sequence,
		payload:          payload,
		metadata:         meta,
	}, nil
}

// EventID returns the globally unique event identifier.
func (e BaseEvent) EventID() string { return e.eventID }

// AggregateID returns the identifier of the emitting aggregate.
func (e BaseEvent) AggregateID() string { return e.aggregateID }

// AggregateType returns the kind of aggregate.
func (e BaseEvent) AggregateType() string { return e.aggregateType }

// EventType returns the versioned event type.
func (e BaseEvent) EventType() string { return e.eventType }

// OccurredAt returns the UTC-normalized emission time.
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }

// AggregateVersion returns the emitting aggregate version.
func (e BaseEvent) AggregateVersion() int64 { return e.aggregateVersion }

// Sequence returns the per-aggregate ordering sequence.
func (e BaseEvent) Sequence() int64 { return e.sequence }

// Payload returns the event-specific data.
func (e BaseEvent) Payload() any { return e.payload }

// Metadata returns a copy of the event metadata; the Custom map is freshly
// copied so callers cannot mutate the envelope.
func (e BaseEvent) Metadata() EventMetadata {
	out := e.metadata
	out.Custom = maps.Clone(e.metadata.Custom)
	return out
}
