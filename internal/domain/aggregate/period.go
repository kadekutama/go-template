package aggregate

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Period is the accounting-period aggregate: open → close → reopen lifecycle
// with caller-verified close preconditions and privileged reopen.
type Period struct {
	data   entity.PeriodData
	events []event.DomainEvent
}

// OpenPeriodParams carries caller-supplied period identity and bounds.
type OpenPeriodParams struct {
	ID         valueobject.PeriodID
	TenantID   valueobject.TenantID
	LedgerID   valueobject.LedgerID
	Start      time.Time
	End        time.Time
	Timezone   string
	Actor      valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// ClosePeriodParams carries caller-verified preconditions: counts the caller
// (E04/E06) confirmed against workflow and reconciliation state. The domain
// stays pure by taking counts, not live queries.
type ClosePeriodParams struct {
	UnresolvedWorkflows int
	UnresolvedBreaks    int
	Actor               valueobject.UserID
	EventID             string
	OccurredAt          time.Time
}

// ReopenPeriodParams carries privileged-reopen approval: both fields required.
type ReopenPeriodParams struct {
	ApprovedBy valueobject.UserID
	Reason     string
	Actor      valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// OpenPeriod creates an OPEN period at version 1 and records period.opened.v1.
func OpenPeriod(p OpenPeriodParams) (Period, error) {
	data := entity.PeriodData{
		ID: p.ID, TenantID: p.TenantID, LedgerID: p.LedgerID,
		Start: p.Start.UTC(), End: p.End.UTC(), Timezone: p.Timezone,
		Status: entity.PeriodOpen, Version: 1,
	}
	if err := data.Validate(); err != nil {
		return Period{}, err
	}
	agg := Period{data: data}
	evt, err := event.NewPeriodOpened(p.EventID, p.ID.String(), p.OccurredAt, 1, 0,
		event.PeriodOpenedPayload{
			PeriodID: p.ID.String(), TenantID: p.TenantID.String(), LedgerID: p.LedgerID.String(),
			Start: data.Start, End: data.End, Timezone: p.Timezone,
		},
		event.EventMetadata{TenantID: p.TenantID.String(), LedgerID: p.LedgerID.String(),
			CausationID: p.EventID, CorrelationID: p.EventID, UserID: p.Actor.String()})
	if err != nil {
		return Period{}, err
	}
	agg.events = append(agg.events, evt)
	return agg, nil
}

// Record returns a copy of the period record.
func (a *Period) Record() entity.PeriodData { return a.data }

// UncommittedEvents returns a copy of events not yet drained to the outbox.
func (a *Period) UncommittedEvents() []event.DomainEvent {
	out := make([]event.DomainEvent, len(a.events))
	copy(out, a.events)
	return out
}

// ClearEvents drains the uncommitted buffer after persistence.
func (a *Period) ClearEvents() { a.events = nil }

func (a *Period) meta(actor valueobject.UserID, eventID string) event.EventMetadata {
	return event.EventMetadata{TenantID: a.data.TenantID.String(), LedgerID: a.data.LedgerID.String(),
		CausationID: eventID, CorrelationID: eventID, UserID: actor.String()}
}

// Close moves OPEN→CLOSED when no unresolved workflows or breaks remain.
func (a *Period) Close(p ClosePeriodParams) error {
	if a.data.Status == entity.PeriodClosed {
		return entity.NewError("PERIOD_CLOSED", "period is already closed")
	}
	if p.UnresolvedWorkflows > 0 || p.UnresolvedBreaks > 0 {
		return entity.Errorf("PERIOD_CLOSE_BLOCKED", "period close blocked: %d unresolved workflows, %d unresolved breaks",
			p.UnresolvedWorkflows, p.UnresolvedBreaks)
	}
	a.data.Status = entity.PeriodClosed
	a.data.Version++
	evt, err := event.NewPeriodClosed(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.PeriodClosedPayload{PeriodID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			ClosedBy: p.Actor.String()},
		a.meta(p.Actor, p.EventID))
	if err != nil {
		return err
	}
	a.events = append(a.events, evt)
	return nil
}

// Reopen moves CLOSED→OPEN as a privileged workflow with approval and reason.
func (a *Period) Reopen(p ReopenPeriodParams) error {
	if a.data.Status == entity.PeriodOpen {
		return entity.NewError("PERIOD_OPEN", "period is already open")
	}
	if p.ApprovedBy.String() == "" || p.Reason == "" {
		return entity.NewError("PERIOD_REOPEN_APPROVAL", "reopen requires an approver and a reason")
	}
	a.data.Status = entity.PeriodOpen
	a.data.Version++
	evt, err := event.NewPeriodReopened(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.PeriodReopenedPayload{PeriodID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			ReopenedBy: p.Actor.String(), Reason: p.Reason},
		a.meta(p.Actor, p.EventID))
	if err != nil {
		return err
	}
	a.events = append(a.events, evt)
	return nil
}
