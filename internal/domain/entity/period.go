package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Period statuses.
const (
	PeriodOpen   = "OPEN"
	PeriodClosed = "CLOSED"
)

// PeriodData is the persistence record for an accounting period: the
// tenant/ledger-scoped window postings are effective in.
type PeriodData struct {
	ID       valueobject.PeriodID
	TenantID valueobject.TenantID
	LedgerID valueobject.LedgerID
	Start    time.Time
	End      time.Time
	Timezone string
	Status   string
	Version  int64
}

// Validate checks period structure.
func (p PeriodData) Validate() error {
	if p.ID.String() == "" {
		return NewError("PERIOD_ID_REQUIRED", "period id is required")
	}
	if p.TenantID.String() == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if p.LedgerID.String() == "" {
		return NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if p.Start.IsZero() || p.End.IsZero() {
		return NewError("PERIOD_BOUNDS_REQUIRED", "period start and end are required")
	}
	if !p.Start.Before(p.End) {
		return NewError("PERIOD_BOUNDS_INVALID", "period start must precede end")
	}
	if p.Timezone == "" {
		return NewError("PERIOD_TIMEZONE_REQUIRED", "accounting timezone is required")
	}
	switch p.Status {
	case PeriodOpen, PeriodClosed:
	default:
		return NewError("PERIOD_STATUS_INVALID", "period status must be OPEN or CLOSED")
	}
	if p.Version < 1 {
		return NewError("PERIOD_VERSION_INVALID", "version starts at 1")
	}
	return nil
}

// Contains reports whether t falls in [Start, End).
func (p PeriodData) Contains(t time.Time) bool {
	return !t.Before(p.Start) && t.Before(p.End)
}
