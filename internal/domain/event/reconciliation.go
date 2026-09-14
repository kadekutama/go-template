package event

import (
	"time"
)

// ReconciliationRunStartedPayload records a reconciliation run trigger.
type ReconciliationRunStartedPayload struct {
	RunID     string    `json:"run_id"`
	TenantID  string    `json:"tenant_id"`
	LedgerID  string    `json:"ledger_id"`
	StartedAt time.Time `json:"started_at"`
}

// NewReconciliationRunStarted builds reconciliation.run.started.v1.
func NewReconciliationRunStarted(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReconciliationRunStartedPayload, meta EventMetadata) (TypedEvent[ReconciliationRunStartedPayload], error) {
	if err := requireIDs(map[string]string{fieldRunID: payload.RunID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[ReconciliationRunStartedPayload]{}, err
	}
	return newTyped("reconciliation.run.started.v1", eventID, aggregateID, "ReconciliationRun", occurredAt, version, seq, payload, meta)
}

// ReconciliationRunCompletedPayload records a finished run.
type ReconciliationRunCompletedPayload struct {
	RunID        string `json:"run_id"`
	TenantID     string `json:"tenant_id"`
	MatchedCount int    `json:"matched_count"`
	BreakCount   int    `json:"break_count"`
}

// NewReconciliationRunCompleted builds reconciliation.run.completed.v1.
func NewReconciliationRunCompleted(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReconciliationRunCompletedPayload, meta EventMetadata) (TypedEvent[ReconciliationRunCompletedPayload], error) {
	if err := requireIDs(map[string]string{fieldRunID: payload.RunID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[ReconciliationRunCompletedPayload]{}, err
	}
	return newTyped("reconciliation.run.completed.v1", eventID, aggregateID, "ReconciliationRun", occurredAt, version, seq, payload, meta)
}

// ReconciliationBreakFoundPayload records a detected mismatch.
type ReconciliationBreakFoundPayload struct {
	BreakID   string `json:"break_id"`
	TenantID  string `json:"tenant_id"`
	RunID     string `json:"run_id"`
	BreakType string `json:"break_type"`
}

// NewReconciliationBreakFound builds reconciliation.break.found.v1.
func NewReconciliationBreakFound(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReconciliationBreakFoundPayload, meta EventMetadata) (TypedEvent[ReconciliationBreakFoundPayload], error) {
	if err := requireIDs(map[string]string{fieldBreakID: payload.BreakID, fieldTenantID: payload.TenantID, fieldRunID: payload.RunID}); err != nil {
		return TypedEvent[ReconciliationBreakFoundPayload]{}, err
	}
	return newTyped("reconciliation.break.found.v1", eventID, aggregateID, "ReconciliationBreak", occurredAt, version, seq, payload, meta)
}

// ReconciliationBreakResolvedPayload records a resolved break.
type ReconciliationBreakResolvedPayload struct {
	BreakID    string `json:"break_id"`
	TenantID   string `json:"tenant_id"`
	ResolvedBy string `json:"resolved_by"`
	Resolution string `json:"resolution"`
}

// NewReconciliationBreakResolved builds reconciliation.break.resolved.v1.
func NewReconciliationBreakResolved(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReconciliationBreakResolvedPayload, meta EventMetadata) (TypedEvent[ReconciliationBreakResolvedPayload], error) {
	if err := requireIDs(map[string]string{fieldBreakID: payload.BreakID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[ReconciliationBreakResolvedPayload]{}, err
	}
	return newTyped("reconciliation.break.resolved.v1", eventID, aggregateID, "ReconciliationBreak", occurredAt, version, seq, payload, meta)
}

// ReconciliationBreakAcknowledgedPayload records an acknowledged break.
type ReconciliationBreakAcknowledgedPayload struct {
	BreakID        string `json:"break_id"`
	TenantID       string `json:"tenant_id"`
	AcknowledgedBy string `json:"acknowledged_by"`
}

// NewReconciliationBreakAcknowledged builds reconciliation.break.acknowledged.v1.
func NewReconciliationBreakAcknowledged(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReconciliationBreakAcknowledgedPayload, meta EventMetadata) (TypedEvent[ReconciliationBreakAcknowledgedPayload], error) {
	if err := requireIDs(map[string]string{fieldBreakID: payload.BreakID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[ReconciliationBreakAcknowledgedPayload]{}, err
	}
	return newTyped("reconciliation.break.acknowledged.v1", eventID, aggregateID, "ReconciliationBreak", occurredAt, version, seq, payload, meta)
}
