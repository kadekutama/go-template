package event

import (
	"time"
)

// PeriodOpenedPayload records a newly created open period.
type PeriodOpenedPayload struct {
	PeriodID string    `json:"period_id"`
	TenantID string    `json:"tenant_id"`
	LedgerID string    `json:"ledger_id"`
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Timezone string    `json:"timezone"`
}

// NewPeriodOpened builds period.opened.v1.
func NewPeriodOpened(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PeriodOpenedPayload, meta EventMetadata) (TypedEvent[PeriodOpenedPayload], error) {
	if err := requireIDs(map[string]string{fieldPeriodID: payload.PeriodID, fieldTenantID: payload.TenantID, fieldLedgerID: payload.LedgerID}); err != nil {
		return TypedEvent[PeriodOpenedPayload]{}, err
	}
	return newTyped("period.opened.v1", eventID, aggregateID, "Period", occurredAt, version, seq, payload, meta)
}

// PeriodClosedPayload records a period close workflow completion.
type PeriodClosedPayload struct {
	PeriodID string `json:"period_id"`
	TenantID string `json:"tenant_id"`
	ClosedBy string `json:"closed_by"`
}

// NewPeriodClosed builds period.closed.v1.
func NewPeriodClosed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PeriodClosedPayload, meta EventMetadata) (TypedEvent[PeriodClosedPayload], error) {
	if err := requireIDs(map[string]string{fieldPeriodID: payload.PeriodID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[PeriodClosedPayload]{}, err
	}
	return newTyped("period.closed.v1", eventID, aggregateID, "Period", occurredAt, version, seq, payload, meta)
}

// PeriodReopenedPayload records a privileged admin reopen.
type PeriodReopenedPayload struct {
	PeriodID   string `json:"period_id"`
	TenantID   string `json:"tenant_id"`
	ReopenedBy string `json:"reopened_by"`
	Reason     string `json:"reason"`
}

// NewPeriodReopened builds period.reopened.v1.
func NewPeriodReopened(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PeriodReopenedPayload, meta EventMetadata) (TypedEvent[PeriodReopenedPayload], error) {
	if err := requireIDs(map[string]string{fieldPeriodID: payload.PeriodID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[PeriodReopenedPayload]{}, err
	}
	return newTyped("period.reopened.v1", eventID, aggregateID, "Period", occurredAt, version, seq, payload, meta)
}
