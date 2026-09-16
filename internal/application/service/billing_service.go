// Package service owns application-level services that are neither commands
// nor queries: cross-cutting metering shared by handlers and workers.
// Billing charges stay with the external provider; this package meters.
package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// UsageKind names one billable event class. Kinds are closed: unknown kinds
// fail closed so new billables are added deliberately, never by typo.
type UsageKind string

// Billable usage kinds.
const (
	UsageTransactionsPosted UsageKind = "transactions-posted"
	UsageAPICalls           UsageKind = "api-calls"
	UsagePayouts            UsageKind = "payouts"
	UsageReportsGenerated   UsageKind = "reports-generated"
)

// usageDayFormat is the UTC bucket layout (YYYY-MM-DD).
const usageDayFormat = "2006-01-02"

// UsageEvent is one billable occurrence. EventID is the idempotency key:
// replays MUST NOT double-count.
type UsageEvent struct {
	EventID    string
	TenantID   string
	Kind       UsageKind
	Quantity   int64
	OccurredAt time.Time
}

// DailyUsage is one tenant/day/kind bucket total.
type DailyUsage struct {
	TenantID string
	Day      string
	Kind     UsageKind
	Quantity int64
}

// UsageStore is the consumer-owned metering persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements. Metering
// is append-only: corrections are new events.
type UsageStore interface {
	// AppendEvent records one event idempotently; applied=false on replay.
	// Day buckets derive from the UTC event date.
	AppendEvent(ctx context.Context, event UsageEvent) (bool, error)
	// DayTotals returns per-kind totals for one tenant day. Strong read.
	DayTotals(ctx context.Context, tenantID, day string) (map[UsageKind]int64, error)
	// PeriodLines returns daily lines over an inclusive day range, ordered by
	// day then kind. Strong read.
	PeriodLines(ctx context.Context, tenantID, fromDay, toDay string) ([]DailyUsage, error)
}

// BillingServiceParams holds dependencies for BillingService.
type BillingServiceParams struct {
	Store UsageStore
}

// BillingService meters billable events and exports invoice-ready CSV
// consumed by the fee-revenue report and external billing providers.
type BillingService struct {
	store UsageStore
}

// NewBillingService constructs a BillingService with injected dependencies.
func NewBillingService(params BillingServiceParams) *BillingService {
	return &BillingService{
		store: params.Store,
	}
}

// Record validates and appends one billable event, reporting whether it
// applied (false on idempotent replay).
func (s *BillingService) Record(ctx context.Context, event UsageEvent) (bool, error) {
	if err := validateUsageEvent(event); err != nil {
		return false, err
	}
	return s.store.AppendEvent(ctx, event)
}

// DayTotals returns per-kind totals for one tenant day.
func (s *BillingService) DayTotals(ctx context.Context, tenantID, day string) (map[UsageKind]int64, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	if _, err := time.Parse(usageDayFormat, day); err != nil {
		return nil, entity.NewError("BILLING_DAY_INVALID", "day must be YYYY-MM-DD")
	}
	return s.store.DayTotals(ctx, tenantID, day)
}

// ExportCSV renders deterministic usage lines plus a TOTAL line over an
// inclusive day range.
func (s *BillingService) ExportCSV(ctx context.Context, tenantID, fromDay, toDay string) ([]byte, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	from, err := time.Parse(usageDayFormat, fromDay)
	if err != nil {
		return nil, entity.NewError("BILLING_DAY_INVALID", "from day must be YYYY-MM-DD")
	}
	to, err := time.Parse(usageDayFormat, toDay)
	if err != nil {
		return nil, entity.NewError("BILLING_DAY_INVALID", "to day must be YYYY-MM-DD")
	}
	if to.Before(from) {
		return nil, entity.NewError("BILLING_RANGE_INVALID", "to day must not precede from day")
	}
	lines, err := s.store.PeriodLines(ctx, tenantID, fromDay, toDay)
	if err != nil {
		return nil, err
	}
	ordered := append([]DailyUsage(nil), lines...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Day != ordered[j].Day {
			return ordered[i].Day < ordered[j].Day
		}
		return ordered[i].Kind < ordered[j].Kind
	})
	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	if err := writer.Write([]string{"tenant", "day", "kind", "quantity"}); err != nil {
		return nil, err
	}
	var total int64
	for _, line := range ordered {
		total += line.Quantity
		if err := writer.Write([]string{line.TenantID, line.Day, string(line.Kind), fmt.Sprintf("%d", line.Quantity)}); err != nil {
			return nil, err
		}
	}
	if err := writer.Write([]string{tenantID, toDay, "TOTAL", fmt.Sprintf("%d", total)}); err != nil {
		return nil, err
	}
	writer.Flush()
	return []byte(buf.String()), writer.Error()
}

// validateUsageEvent checks the metering envelope including kind membership.
func validateUsageEvent(event UsageEvent) error {
	if strings.TrimSpace(event.EventID) == "" {
		return entity.NewError("USAGE_EVENT_REQUIRED", "usage event id is required")
	}
	if strings.TrimSpace(event.TenantID) == "" {
		return entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	switch event.Kind {
	case UsageTransactionsPosted, UsageAPICalls, UsagePayouts, UsageReportsGenerated:
	default:
		return entity.NewError("USAGE_KIND_UNKNOWN", "usage kind is unknown")
	}
	if event.Quantity <= 0 {
		return entity.NewError("USAGE_QUANTITY_INVALID", "usage quantity must be positive")
	}
	if event.OccurredAt.IsZero() {
		return entity.NewError("USAGE_TIME_REQUIRED", "usage time is required")
	}
	return nil
}
