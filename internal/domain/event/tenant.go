package event

import (
	"errors"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// TenantUpdatedPayload describes a tenant settings change. It carries the
// replacement settings so the audit trail records what changed.
type TenantUpdatedPayload struct {
	TenantID  string                `json:"tenant_id"`
	Name      string                `json:"name"`
	Region    string                `json:"region"`
	Settings  entity.TenantSettings `json:"settings"`
	UpdatedBy string                `json:"updated_by"`
}

// NewTenantUpdated builds tenant.updated.v1.
func NewTenantUpdated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TenantUpdatedPayload, meta EventMetadata) (TypedEvent[TenantUpdatedPayload], error) {
	if err := requireIDs(map[string]string{fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TenantUpdatedPayload]{}, err
	}
	return newTyped("tenant.updated.v1", eventID, aggregateID, "Tenant", occurredAt, version, seq, payload, meta)
}

// TenantSuspendedPayload describes a tenant suspension.
type TenantSuspendedPayload struct {
	TenantID    string `json:"tenant_id"`
	Reason      string `json:"reason"`
	SuspendedBy string `json:"suspended_by"`
}

// NewTenantSuspended builds tenant.suspended.v1.
func NewTenantSuspended(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TenantSuspendedPayload, meta EventMetadata) (TypedEvent[TenantSuspendedPayload], error) {
	if err := requireIDs(map[string]string{fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TenantSuspendedPayload]{}, err
	}
	if strings.TrimSpace(payload.Reason) == "" {
		return TypedEvent[TenantSuspendedPayload]{}, errors.New("event: reason is required")
	}
	return newTyped("tenant.suspended.v1", eventID, aggregateID, "Tenant", occurredAt, version, seq, payload, meta)
}

// TenantReactivatedPayload describes a tenant reactivation.
type TenantReactivatedPayload struct {
	TenantID      string `json:"tenant_id"`
	ReactivatedBy string `json:"reactivated_by"`
}

// NewTenantReactivated builds tenant.reactivated.v1.
func NewTenantReactivated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TenantReactivatedPayload, meta EventMetadata) (TypedEvent[TenantReactivatedPayload], error) {
	if err := requireIDs(map[string]string{fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TenantReactivatedPayload]{}, err
	}
	return newTyped("tenant.reactivated.v1", eventID, aggregateID, "Tenant", occurredAt, version, seq, payload, meta)
}

// TenantClosedPayload describes a tenant closure.
type TenantClosedPayload struct {
	TenantID string `json:"tenant_id"`
	Reason   string `json:"reason"`
	ClosedBy string `json:"closed_by"`
}

// NewTenantClosed builds tenant.closed.v1.
func NewTenantClosed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TenantClosedPayload, meta EventMetadata) (TypedEvent[TenantClosedPayload], error) {
	if err := requireIDs(map[string]string{fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TenantClosedPayload]{}, err
	}
	if strings.TrimSpace(payload.Reason) == "" {
		return TypedEvent[TenantClosedPayload]{}, errors.New("event: reason is required")
	}
	return newTyped("tenant.closed.v1", eventID, aggregateID, "Tenant", occurredAt, version, seq, payload, meta)
}
