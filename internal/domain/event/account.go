package event

import (
	"errors"
	"time"
)

// AccountCreatedPayload describes a newly opened account.
type AccountCreatedPayload struct {
	AccountID     string `json:"account_id"`
	TenantID      string `json:"tenant_id"`
	AccountNumber string `json:"account_number"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	AssetCode     string `json:"asset_code"`
	Status        string `json:"status"`
	OpenedBy      string `json:"opened_by"`
}

// NewAccountCreated builds account.created.v1.
func NewAccountCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountCreatedPayload, meta EventMetadata) (TypedEvent[AccountCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID, "account_number": payload.AccountNumber}); err != nil {
		return TypedEvent[AccountCreatedPayload]{}, err
	}
	return newTyped("account.created.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// AccountUpdatedPayload describes an account metadata update.
type AccountUpdatedPayload struct {
	AccountID string `json:"account_id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	UpdatedBy string `json:"updated_by"`
}

// NewAccountUpdated builds account.updated.v1.
func NewAccountUpdated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountUpdatedPayload, meta EventMetadata) (TypedEvent[AccountUpdatedPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[AccountUpdatedPayload]{}, err
	}
	return newTyped("account.updated.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// AccountFrozenPayload describes an account freeze.
type AccountFrozenPayload struct {
	AccountID string `json:"account_id"`
	TenantID  string `json:"tenant_id"`
	Reason    string `json:"reason"`
	FrozenBy  string `json:"frozen_by"`
}

// NewAccountFrozen builds account.frozen.v1.
func NewAccountFrozen(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountFrozenPayload, meta EventMetadata) (TypedEvent[AccountFrozenPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[AccountFrozenPayload]{}, err
	}
	if payload.Reason == "" {
		return TypedEvent[AccountFrozenPayload]{}, errors.New("event: reason is required")
	}
	return newTyped("account.frozen.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// AccountUnfrozenPayload describes an account unfreeze.
type AccountUnfrozenPayload struct {
	AccountID  string `json:"account_id"`
	TenantID   string `json:"tenant_id"`
	UnfrozenBy string `json:"unfrozen_by"`
}

// NewAccountUnfrozen builds account.unfrozen.v1.
func NewAccountUnfrozen(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountUnfrozenPayload, meta EventMetadata) (TypedEvent[AccountUnfrozenPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[AccountUnfrozenPayload]{}, err
	}
	return newTyped("account.unfrozen.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// AccountClosedPayload describes an account closure.
type AccountClosedPayload struct {
	AccountID string `json:"account_id"`
	TenantID  string `json:"tenant_id"`
	Reason    string `json:"reason"`
	ClosedBy  string `json:"closed_by"`
}

// NewAccountClosed builds account.closed.v1.
func NewAccountClosed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountClosedPayload, meta EventMetadata) (TypedEvent[AccountClosedPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[AccountClosedPayload]{}, err
	}
	if payload.Reason == "" {
		return TypedEvent[AccountClosedPayload]{}, errors.New("event: reason is required")
	}
	return newTyped("account.closed.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// AccountVerifiedPayload describes confirmed microdeposit verification.
type AccountVerifiedPayload struct {
	AccountID  string `json:"account_id"`
	TenantID   string `json:"tenant_id"`
	VerifiedBy string `json:"verified_by"`
}

// NewAccountVerified builds account.verified.v1.
func NewAccountVerified(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload AccountVerifiedPayload, meta EventMetadata) (TypedEvent[AccountVerifiedPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[AccountVerifiedPayload]{}, err
	}
	return newTyped("account.verified.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// BalanceChangedPayload carries per-account balance dimensions at a cursor.
type BalanceChangedPayload struct {
	AccountID      string    `json:"account_id"`
	TenantID       string    `json:"tenant_id"`
	AssetCode      string    `json:"asset_code"`
	PostedMinor    int64     `json:"posted_minor"`
	AvailableMinor int64     `json:"available_minor"`
	HeldMinor      int64     `json:"held_minor"`
	PendingMinor   int64     `json:"pending_minor"`
	ReservedMinor  int64     `json:"reserved_minor"`
	LedgerCursor   int64     `json:"ledger_cursor"`
	AsOf           time.Time `json:"as_of"`
	ReferenceType  string    `json:"reference_type"`
	ReferenceID    string    `json:"reference_id"`
}

// NewBalanceChanged builds account.balance.changed.v1.
func NewBalanceChanged(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload BalanceChangedPayload, meta EventMetadata) (TypedEvent[BalanceChangedPayload], error) {
	if err := requireIDs(map[string]string{fieldAccountID: payload.AccountID, fieldTenantID: payload.TenantID, "asset_code": payload.AssetCode}); err != nil {
		return TypedEvent[BalanceChangedPayload]{}, err
	}
	return newTyped("account.balance.changed.v1", eventID, aggregateID, "Account", occurredAt, version, seq, payload, meta)
}

// TenantCreatedPayload describes committed tenant onboarding.
type TenantCreatedPayload struct {
	TenantID      string    `json:"tenant_id"`
	LedgerID      string    `json:"ledger_id"`
	Name          string    `json:"name"`
	Region        string    `json:"region"`
	BaseAssetCode string    `json:"base_asset_code"`
	CreatedAt     time.Time `json:"created_at"`
}

// NewTenantCreated builds tenant.created.v1.
func NewTenantCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TenantCreatedPayload, meta EventMetadata) (TypedEvent[TenantCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldTenantID: payload.TenantID, fieldLedgerID: payload.LedgerID}); err != nil {
		return TypedEvent[TenantCreatedPayload]{}, err
	}
	return newTyped("tenant.created.v1", eventID, aggregateID, "Tenant", occurredAt, version, seq, payload, meta)
}

// PIIErasedPayload is an internal control-plane record. It carries no erased
// value and must not become a public webhook without a separately approved
// projection.
type PIIErasedPayload struct {
	ErasureID     string    `json:"erasure_id"`
	TenantID      string    `json:"tenant_id"`
	SubjectType   string    `json:"subject_type"`
	SubjectID     string    `json:"subject_id"`
	PolicyVersion string    `json:"policy_version"`
	ErasedAt      time.Time `json:"erased_at"`
}

// NewPIIErased builds pii.erased.v1.
func NewPIIErased(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PIIErasedPayload, meta EventMetadata) (TypedEvent[PIIErasedPayload], error) {
	if err := requireIDs(map[string]string{"erasure_id": payload.ErasureID, fieldTenantID: payload.TenantID, "subject_id": payload.SubjectID}); err != nil {
		return TypedEvent[PIIErasedPayload]{}, err
	}
	return newTyped("pii.erased.v1", eventID, aggregateID, "PrivacySubject", occurredAt, version, seq, payload, meta)
}
