package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Hold states.
const (
	HoldActive   = "ACTIVE"
	HoldCaptured = "CAPTURED"
	HoldReleased = "RELEASED"
	HoldExpired  = "EXPIRED"
)

// HoldData is the persistence record for a durable hold: an expiring
// authorization/reservation that changes available balance but is not a posted
// accounting fact and never appears in an entry set.
type HoldData struct {
	ID          valueobject.HoldID
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	AccountID   valueobject.AccountID
	AssetCode   valueobject.AssetCode
	AmountMinor int64
	Kind        string
	State       string
	ExpiresAt   time.Time
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Validate checks hold structure. An empty ID is permitted prior to persistence.
func (h HoldData) Validate() error {
	if h.ID.String() != "" {
		if _, err := valueobject.ParseHoldID(h.ID.String()); err != nil {
			return NewError("HOLD_ID_INVALID", "hold id is invalid")
		}
	}
	if h.TenantID.String() == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if h.LedgerID.String() == "" {
		return NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if h.AccountID.String() == "" {
		return NewError("HOLD_ACCOUNT_REQUIRED", "account id is required")
	}
	if h.AssetCode == "" {
		return NewError("HOLD_ASSET_REQUIRED", "asset code is required")
	}
	if h.AmountMinor <= 0 {
		return NewError("HOLD_AMOUNT_INVALID", "hold amount must be positive")
	}
	if h.Kind == "" {
		return NewError("HOLD_KIND_REQUIRED", "hold kind is required")
	}
	switch h.State {
	case HoldActive, HoldCaptured, HoldReleased, HoldExpired:
	default:
		return NewError("HOLD_STATE_INVALID", "hold state is invalid")
	}
	if h.ExpiresAt.IsZero() {
		return NewError("HOLD_EXPIRY_REQUIRED", "expiry is required")
	}
	if h.Version < 1 {
		return NewError("HOLD_VERSION_INVALID", "version starts at 1")
	}
	return nil
}
