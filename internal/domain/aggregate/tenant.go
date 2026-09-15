package aggregate

import (
	"slices"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Tenant is the tenancy aggregate root: identity, region, lifecycle, and
// settings. It owns no balance and no secrets.
type Tenant struct {
	data   entity.TenantData
	events []event.DomainEvent
}

// OpenTenantParams carries explicit caller-supplied identity and scope.
type OpenTenantParams struct {
	ID         valueobject.TenantID
	LedgerID   valueobject.LedgerID
	Name       string
	Region     string
	Settings   entity.TenantSettings
	OpenedBy   valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// TenantTransitionParams carries actor and envelope identity for mutations.
type TenantTransitionParams struct {
	Actor      valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// TenantSuspendParams carries the suspension reason.
type TenantSuspendParams struct {
	TenantTransitionParams
	Reason string
}

// TenantCloseParams carries the closure reason.
type TenantCloseParams struct {
	TenantTransitionParams
	Reason string
}

// TenantUpdateSettingsParams carries replacement settings.
type TenantUpdateSettingsParams struct {
	TenantTransitionParams
	Settings entity.TenantSettings
}

func tenantMeta(t *Tenant, tr TenantTransitionParams, ledgerID valueobject.LedgerID) event.EventMetadata {
	return event.EventMetadata{
		TenantID:      t.data.ID.String(),
		LedgerID:      ledgerID.String(),
		CausationID:   tr.EventID,
		CorrelationID: tr.EventID,
		UserID:        tr.Actor.String(),
	}
}

// OpenTenant creates an ACTIVE tenant at version 1 and records
// tenant.created.v1.
func OpenTenant(p OpenTenantParams) (Tenant, error) {
	if p.ID.String() == "" {
		return Tenant{}, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	if p.LedgerID.String() == "" {
		return Tenant{}, entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if p.OpenedBy.String() == "" {
		return Tenant{}, entity.NewError("OPENED_BY_REQUIRED", "opened by user id is required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return Tenant{}, entity.NewError("EVENT_ID_REQUIRED", "event id is required")
	}
	if p.OccurredAt.IsZero() {
		return Tenant{}, entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required")
	}
	data := entity.TenantData{
		ID:        p.ID,
		Name:      p.Name,
		Region:    p.Region,
		Status:    entity.TenantActive,
		Settings:  cloneTenantSettings(p.Settings),
		Version:   1,
		CreatedAt: p.OccurredAt.UTC(),
		UpdatedAt: p.OccurredAt.UTC(),
	}
	if err := data.Validate(); err != nil {
		return Tenant{}, err
	}
	t := Tenant{data: data}
	evt, err := event.NewTenantCreated(p.EventID, p.ID.String(), p.OccurredAt, 1, 0,
		event.TenantCreatedPayload{
			TenantID:      p.ID.String(),
			LedgerID:      p.LedgerID.String(),
			Name:          p.Name,
			Region:        p.Region,
			BaseAssetCode: string(p.Settings.DefaultCurrency),
			CreatedAt:     p.OccurredAt.UTC(),
		},
		event.EventMetadata{TenantID: p.ID.String(), LedgerID: p.LedgerID.String(),
			CausationID: p.EventID, CorrelationID: p.EventID, UserID: p.OpenedBy.String()})
	if err != nil {
		return Tenant{}, err
	}
	t.append(evt)
	return t, nil
}

// Record returns a defensive copy of the tenant record.
func (t *Tenant) Record() entity.TenantData {
	out := t.data
	out.Settings.EnabledFeatures = slices.Clone(t.data.Settings.EnabledFeatures)
	out.Settings.EnabledPaymentMethods = slices.Clone(t.data.Settings.EnabledPaymentMethods)
	return out
}

// UncommittedEvents returns a copy of events not yet drained to the outbox.
func (t *Tenant) UncommittedEvents() []event.DomainEvent {
	out := make([]event.DomainEvent, len(t.events))
	copy(out, t.events)
	return out
}

// ClearEvents drains the uncommitted buffer after persistence.
func (t *Tenant) ClearEvents() { t.events = nil }

func (t *Tenant) append(ev event.DomainEvent) {
	t.events = append(t.events, ev)
}

func (t *Tenant) requireOpen() error {
	if t.data.Status == entity.TenantClosed {
		return entity.NewError("TENANT_CLOSED", "tenant is closed")
	}
	return nil
}

// Suspend moves ACTIVE→SUSPENDED and records tenant.suspended.v1.
func (t *Tenant) Suspend(p TenantSuspendParams, ledgerID valueobject.LedgerID) error {
	if err := t.requireOpen(); err != nil {
		return err
	}
	if t.data.Status == entity.TenantSuspended {
		return entity.NewError("TENANT_ALREADY_SUSPENDED", "tenant is already suspended")
	}
	trimmedReason := strings.TrimSpace(p.Reason)
	if trimmedReason == "" {
		return entity.NewError("TENANT_REASON_REQUIRED", "suspend reason is required")
	}
	if ledgerID.String() == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return entity.NewError("EVENT_ID_REQUIRED", "event id is required")
	}
	if p.OccurredAt.IsZero() {
		return entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required")
	}
	newVersion := t.data.Version + 1
	evt, err := event.NewTenantSuspended(p.EventID, t.data.ID.String(), p.OccurredAt,
		newVersion, int64(len(t.events)),
		event.TenantSuspendedPayload{TenantID: t.data.ID.String(), Reason: trimmedReason, SuspendedBy: p.Actor.String()},
		tenantMeta(t, p.TenantTransitionParams, ledgerID))
	if err != nil {
		return err
	}
	t.data.Status = entity.TenantSuspended
	t.data.Version = newVersion
	t.data.UpdatedAt = p.OccurredAt.UTC()
	t.append(evt)
	return nil
}

// Reactivate moves SUSPENDED→ACTIVE and records tenant.reactivated.v1.
func (t *Tenant) Reactivate(p TenantTransitionParams, ledgerID valueobject.LedgerID) error {
	if err := t.requireOpen(); err != nil {
		return err
	}
	if t.data.Status != entity.TenantSuspended {
		return entity.NewError("TENANT_NOT_SUSPENDED", "tenant is not suspended")
	}
	if ledgerID.String() == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return entity.NewError("EVENT_ID_REQUIRED", "event id is required")
	}
	if p.OccurredAt.IsZero() {
		return entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required")
	}
	newVersion := t.data.Version + 1
	evt, err := event.NewTenantReactivated(p.EventID, t.data.ID.String(), p.OccurredAt,
		newVersion, int64(len(t.events)),
		event.TenantReactivatedPayload{TenantID: t.data.ID.String(), ReactivatedBy: p.Actor.String()},
		tenantMeta(t, p, ledgerID))
	if err != nil {
		return err
	}
	t.data.Status = entity.TenantActive
	t.data.Version = newVersion
	t.data.UpdatedAt = p.OccurredAt.UTC()
	t.append(evt)
	return nil
}

// Close moves ACTIVE/SUSPENDED→CLOSED (terminal) and records tenant.closed.v1.
func (t *Tenant) Close(p TenantCloseParams, ledgerID valueobject.LedgerID) error {
	if err := t.requireOpen(); err != nil {
		return err
	}
	trimmedReason := strings.TrimSpace(p.Reason)
	if trimmedReason == "" {
		return entity.NewError("TENANT_REASON_REQUIRED", "close reason is required")
	}
	if ledgerID.String() == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return entity.NewError("EVENT_ID_REQUIRED", "event id is required")
	}
	if p.OccurredAt.IsZero() {
		return entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required")
	}
	newVersion := t.data.Version + 1
	evt, err := event.NewTenantClosed(p.EventID, t.data.ID.String(), p.OccurredAt,
		newVersion, int64(len(t.events)),
		event.TenantClosedPayload{TenantID: t.data.ID.String(), Reason: trimmedReason, ClosedBy: p.Actor.String()},
		tenantMeta(t, p.TenantTransitionParams, ledgerID))
	if err != nil {
		return err
	}
	t.data.Status = entity.TenantClosed
	t.data.Version = newVersion
	t.data.UpdatedAt = p.OccurredAt.UTC()
	t.append(evt)
	return nil
}

// UpdateSettings replaces settings (not on CLOSED) and records
// tenant.updated.v1.
func (t *Tenant) UpdateSettings(p TenantUpdateSettingsParams, ledgerID valueobject.LedgerID) error {
	if err := t.requireOpen(); err != nil {
		return err
	}
	if err := p.Settings.Validate(); err != nil {
		return err
	}
	if ledgerID.String() == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(p.EventID) == "" {
		return entity.NewError("EVENT_ID_REQUIRED", "event id is required")
	}
	if p.OccurredAt.IsZero() {
		return entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required")
	}
	cloned := cloneTenantSettings(p.Settings)
	newVersion := t.data.Version + 1
	evt, err := event.NewTenantUpdated(p.EventID, t.data.ID.String(), p.OccurredAt,
		newVersion, int64(len(t.events)),
		event.TenantUpdatedPayload{TenantID: t.data.ID.String(), Name: t.data.Name, Region: t.data.Region, Settings: cloneTenantSettings(cloned), UpdatedBy: p.Actor.String()},
		tenantMeta(t, p.TenantTransitionParams, ledgerID))
	if err != nil {
		return err
	}
	t.data.Settings = cloned
	t.data.Version = newVersion
	t.data.UpdatedAt = p.OccurredAt.UTC()
	t.append(evt)
	return nil
}

func cloneTenantSettings(s entity.TenantSettings) entity.TenantSettings {
	out := s
	out.EnabledFeatures = slices.Clone(s.EnabledFeatures)
	out.EnabledPaymentMethods = slices.Clone(s.EnabledPaymentMethods)
	return out
}
