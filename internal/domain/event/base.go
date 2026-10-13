// Package event provides the immutable domain event envelope consumed by
// every aggregate in E02-T03–T05 and cataloged in E02-T06.
package event

import (
	"maps"
	"time"
)

// Metadata carries tenant scoping and correlation context for a domain event.
type Metadata struct {
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
	Metadata() Metadata
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
	metadata         Metadata
}

// Compile-time proof that BaseEvent satisfies DomainEvent.
var _ DomainEvent = BaseEvent{}

// NewBaseEvent validates its inputs and returns an immutable BaseEvent.
// IDs, types, tenant scope, and timestamps are caller-supplied (application
// layer via the kernel IDGenerator/Clock); the domain never generates them.
// The custom metadata map is defensively copied. It returns an error on
// invalid input and never panics.
func NewBaseEvent(eventID, aggregateID, aggregateType, eventType string, occurredAt time.Time, aggregateVersion, sequence int64, payload any, meta Metadata) (BaseEvent, error) {
	if eventID == "" {
		return BaseEvent{}, ErrEventEventIDIsRequired
	}

	if aggregateID == "" {
		return BaseEvent{}, ErrEventAggregateIDIsRequired
	}

	if aggregateType == "" {
		return BaseEvent{}, ErrEventAggregateTypeIsRequired
	}

	if eventType == "" {
		return BaseEvent{}, ErrEventEventTypeIsRequired
	}

	if meta.TenantID == "" {
		return BaseEvent{}, ErrEventMetadataTenantIDIsRequired
	}

	if occurredAt.IsZero() {
		return BaseEvent{}, ErrEventOccurredAtIsRequired
	}

	if aggregateVersion < 0 {
		return BaseEvent{}, ErrEventAggregateVersionMustNotBeNegative
	}

	if sequence < 0 {
		return BaseEvent{}, ErrEventSequenceMustNotBeNegative
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
func (e BaseEvent) Metadata() Metadata {
	out := e.metadata
	out.Custom = maps.Clone(e.metadata.Custom)

	return out
}
