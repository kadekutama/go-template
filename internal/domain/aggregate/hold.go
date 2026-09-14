package aggregate

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Hold is the durable authorization/reservation aggregate. It changes
// available balance but is never an entry and emits no posting events.
type Hold struct {
	data entity.HoldData
}

// OpenHoldParams carries caller-supplied hold identity and scope.
type OpenHoldParams struct {
	ID          valueobject.HoldID
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	AccountID   valueobject.AccountID
	AssetCode   valueobject.AssetCode
	AmountMinor int64
	Kind        string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// OpenHold creates an ACTIVE hold at version 1.
func OpenHold(p OpenHoldParams) (Hold, error) {
	data := entity.HoldData{
		ID: p.ID, TenantID: p.TenantID, LedgerID: p.LedgerID, AccountID: p.AccountID,
		AssetCode: p.AssetCode, AmountMinor: p.AmountMinor, Kind: p.Kind,
		State: entity.HoldActive, ExpiresAt: p.ExpiresAt.UTC(), Version: 1,
		CreatedAt: p.CreatedAt.UTC(), UpdatedAt: p.CreatedAt.UTC(),
	}
	if err := data.Validate(); err != nil {
		return Hold{}, err
	}
	return Hold{data: data}, nil
}

// Record returns a copy of the hold record.
func (h *Hold) Record() entity.HoldData { return h.data }

// State returns the current hold state.
func (h *Hold) State() string { return h.data.State }

func (h *Hold) touch(at time.Time) {
	h.data.Version++
	h.data.UpdatedAt = at.UTC()
}

// Capture moves ACTIVE→CAPTURED. Repeating capture on CAPTURED is idempotent;
// capture after expiry or from RELEASED/EXPIRED fails.
func (h *Hold) Capture(at time.Time) error {
	switch h.data.State {
	case entity.HoldCaptured:
		return nil
	case entity.HoldActive:
	default:
		return entity.Errorf("HOLD_STATE_CONFLICT", "cannot capture hold in state %s", h.data.State)
	}
	if !at.UTC().Before(h.data.ExpiresAt) {
		return entity.NewError("HOLD_EXPIRED", "hold has expired")
	}
	h.data.State = entity.HoldCaptured
	h.touch(at)
	return nil
}

// Release moves ACTIVE→RELEASED. Repeating release on RELEASED is idempotent;
// release from CAPTURED/EXPIRED fails.
func (h *Hold) Release(at time.Time) error {
	switch h.data.State {
	case entity.HoldReleased:
		return nil
	case entity.HoldActive:
	default:
		return entity.Errorf("HOLD_STATE_CONFLICT", "cannot release hold in state %s", h.data.State)
	}
	h.data.State = entity.HoldReleased
	h.touch(at)
	return nil
}

// Expire moves ACTIVE→EXPIRED once ExpiresAt passes. Repeating expiry on
// EXPIRED is idempotent; early expiry fails.
func (h *Hold) Expire(at time.Time) error {
	switch h.data.State {
	case entity.HoldExpired:
		return nil
	case entity.HoldActive:
	default:
		return entity.Errorf("HOLD_STATE_CONFLICT", "cannot expire hold in state %s", h.data.State)
	}
	if at.UTC().Before(h.data.ExpiresAt) {
		return entity.NewError("HOLD_NOT_EXPIRED", "hold has not expired yet")
	}
	h.data.State = entity.HoldExpired
	h.touch(at)
	return nil
}
