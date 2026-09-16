// Package aggregate holds aggregate roots: consistency boundaries that own
// invariants, versioning, and uncommitted domain events.
package aggregate

import (
	"maps"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Account is the ledger account aggregate root: classification, hierarchy,
// and lifecycle. It owns no balance.
type Account struct {
	data   entity.AccountData
	events []event.DomainEvent
}

// OpenAccountParams carries explicit caller-supplied identity and scope.
type OpenAccountParams struct {
	ID         valueobject.AccountID
	TenantID   valueobject.TenantID
	LedgerID   valueobject.LedgerID
	ParentID   *valueobject.AccountID
	Number     string
	Name       string
	Class      valueobject.AccountClass
	AssetCode  valueobject.AssetCode
	Purpose    string
	Metadata   map[string]string
	OpenedBy   valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// TransitionParams carries actor and envelope identity for mutations.
type TransitionParams struct {
	Actor      valueobject.UserID
	EventID    string
	OccurredAt time.Time
}

// FreezeParams carries the freeze reason.
type FreezeParams struct {
	TransitionParams
	Reason string
}

// CloseParams carries the closure reason.
type CloseParams struct {
	TransitionParams
	Reason string
}

// ReparentParams carries the new parent (nil detaches to root).
type ReparentParams struct {
	TransitionParams
	NewParent *valueobject.AccountID
}

// UpdateDetailsParams carries editable metadata.
type UpdateDetailsParams struct {
	TransitionParams
	Name     string
	Purpose  string
	Metadata map[string]string
}

func envelopeMeta(a *Account, tr TransitionParams) event.EventMetadata {
	return event.EventMetadata{
		TenantID:      a.data.TenantID.String(),
		LedgerID:      a.data.LedgerID.String(),
		CausationID:   tr.EventID,
		CorrelationID: tr.EventID,
		UserID:        tr.Actor.String(),
	}
}

func (a *Account) append(ev event.DomainEvent) {
	a.events = append(a.events, ev)
}

// OpenAccount creates an ACTIVE account at version 1 and records
// account.created.v1.
func OpenAccount(p OpenAccountParams) (Account, error) {
	data := entity.AccountData{
		ID: p.ID, TenantID: p.TenantID, LedgerID: p.LedgerID, ParentID: p.ParentID,
		Number: p.Number, Name: p.Name, Class: p.Class, AssetCode: p.AssetCode,
		Status: valueobject.StatusActive, Purpose: p.Purpose,
		Metadata: maps.Clone(p.Metadata), Version: 1,
		CreatedAt: p.OccurredAt.UTC(), UpdatedAt: p.OccurredAt.UTC(),
	}
	if p.ParentID != nil && *p.ParentID == p.ID {
		return Account{}, entity.NewError("ACCOUNT_PARENT_INVALID", "account cannot parent to itself")
	}
	if err := data.Validate(); err != nil {
		return Account{}, err
	}
	a := Account{data: data}
	evt, err := event.NewAccountCreated(p.EventID, p.ID.String(), p.OccurredAt, 1, 0,
		event.AccountCreatedPayload{
			AccountID: p.ID.String(), TenantID: p.TenantID.String(),
			AccountNumber: p.Number, Name: p.Name, Type: string(p.Class),
			AssetCode: string(p.AssetCode), Status: string(valueobject.StatusActive),
			OpenedBy: p.OpenedBy.String(),
		},
		event.EventMetadata{TenantID: p.TenantID.String(), LedgerID: p.LedgerID.String(),
			CausationID: p.EventID, CorrelationID: p.EventID, UserID: p.OpenedBy.String()})
	if err != nil {
		return Account{}, err
	}
	a.append(evt)
	return a, nil
}

// Record returns a defensive copy of the account record.
func (a *Account) Record() entity.AccountData {
	out := a.data
	out.Metadata = maps.Clone(a.data.Metadata)
	return out
}

// UncommittedEvents returns a copy of events not yet drained to the outbox.
func (a *Account) UncommittedEvents() []event.DomainEvent {
	out := make([]event.DomainEvent, len(a.events))
	copy(out, a.events)
	return out
}

// LoadAccount rehydrates an aggregate from its stored record for command
// handling (added in E06-T02: no transition path existed for stored
// accounts). Events start empty: only new transitions are uncommitted.
func LoadAccount(data entity.AccountData) Account {
	return Account{data: data}
}

// ClearEvents drains the uncommitted buffer after persistence.
func (a *Account) ClearEvents() { a.events = nil }

func (a *Account) bump(at time.Time) {
	a.data.Version++
	a.data.UpdatedAt = at.UTC()
}

func (a *Account) requireOpen() error {
	if a.data.Status == valueobject.StatusClosed {
		return entity.NewError("ACCOUNT_CLOSED", "account is closed")
	}
	return nil
}

// Freeze moves ACTIVE→FROZEN and records account.frozen.v1.
func (a *Account) Freeze(p FreezeParams) error {
	if err := a.requireOpen(); err != nil {
		return err
	}
	if a.data.Status == valueobject.StatusFrozen {
		return entity.NewError("ACCOUNT_ALREADY_FROZEN", "account is already frozen")
	}
	if p.Reason == "" {
		return entity.NewError("ACCOUNT_REASON_REQUIRED", "freeze reason is required")
	}
	a.data.Status = valueobject.StatusFrozen
	a.bump(p.OccurredAt)
	evt, err := event.NewAccountFrozen(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.AccountFrozenPayload{AccountID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			Reason: p.Reason, FrozenBy: p.Actor.String()},
		envelopeMeta(a, p.TransitionParams))
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}

// Unfreeze moves FROZEN→ACTIVE and records account.unfrozen.v1.
func (a *Account) Unfreeze(p TransitionParams) error {
	if err := a.requireOpen(); err != nil {
		return err
	}
	if a.data.Status != valueobject.StatusFrozen {
		return entity.NewError("ACCOUNT_NOT_FROZEN", "account is not frozen")
	}
	a.data.Status = valueobject.StatusActive
	a.bump(p.OccurredAt)
	evt, err := event.NewAccountUnfrozen(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.AccountUnfrozenPayload{AccountID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			UnfrozenBy: p.Actor.String()},
		envelopeMeta(a, p))
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}

// Close moves ACTIVE/FROZEN→CLOSED (terminal) and records account.closed.v1.
func (a *Account) Close(p CloseParams) error {
	if err := a.requireOpen(); err != nil {
		return err
	}
	if p.Reason == "" {
		return entity.NewError("ACCOUNT_REASON_REQUIRED", "close reason is required")
	}
	a.data.Status = valueobject.StatusClosed
	a.bump(p.OccurredAt)
	evt, err := event.NewAccountClosed(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.AccountClosedPayload{AccountID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			Reason: p.Reason, ClosedBy: p.Actor.String()},
		envelopeMeta(a, p.TransitionParams))
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}

// Reparent changes hierarchy (not on CLOSED) and records account.updated.v1.
func (a *Account) Reparent(p ReparentParams) error {
	if err := a.requireOpen(); err != nil {
		return err
	}
	if p.NewParent != nil && *p.NewParent == a.data.ID {
		return entity.NewError("ACCOUNT_PARENT_INVALID", "account cannot parent to itself")
	}
	a.data.ParentID = p.NewParent
	a.bump(p.OccurredAt)
	evt, err := event.NewAccountUpdated(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.AccountUpdatedPayload{AccountID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			Name: a.data.Name, UpdatedBy: p.Actor.String()},
		envelopeMeta(a, p.TransitionParams))
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}

// UpdateDetails changes name/purpose/metadata (not on CLOSED) and records
// account.updated.v1.
func (a *Account) UpdateDetails(p UpdateDetailsParams) error {
	if err := a.requireOpen(); err != nil {
		return err
	}
	if p.Name == "" {
		return entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required")
	}
	a.data.Name = p.Name
	a.data.Purpose = p.Purpose
	a.data.Metadata = maps.Clone(p.Metadata)
	a.bump(p.OccurredAt)
	evt, err := event.NewAccountUpdated(p.EventID, a.data.ID.String(), p.OccurredAt,
		a.data.Version, int64(len(a.events)),
		event.AccountUpdatedPayload{AccountID: a.data.ID.String(), TenantID: a.data.TenantID.String(),
			Name: p.Name, UpdatedBy: p.Actor.String()},
		envelopeMeta(a, p.TransitionParams))
	if err != nil {
		return err
	}
	a.append(evt)
	return nil
}
