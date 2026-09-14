package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Dispute is the dispute workflow aggregate: evidence tracking,
// representment stage, and linkage to the original posting, hold, and fee.
type Dispute struct {
	ID                string
	PaymentID         string
	OriginalPostingID string
	Network           string
	AmountMinor       int64
	OpenedAt          time.Time
	EvidenceDueAt     time.Time
	Status            valueobject.DisputeStatus
	HoldID            string
	FeeMinor          int64
	RepresentStage    int
	PolicyVersion     string
}

// Validate checks dispute identity and linkage.
func (d Dispute) Validate() error {
	if d.ID == "" || d.PaymentID == "" || d.OriginalPostingID == "" {
		return NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting")
	}
	if d.Network == "" {
		return NewError("DISPUTE_NETWORK_REQUIRED", "dispute requires a network")
	}
	if d.AmountMinor <= 0 {
		return NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive")
	}
	if d.OpenedAt.IsZero() || d.EvidenceDueAt.IsZero() || !d.EvidenceDueAt.After(d.OpenedAt) {
		return NewError("DISPUTE_WINDOW_INVALID", "dispute requires an evidence window after opening")
	}
	if _, err := valueobject.ParseDisputeStatus(string(d.Status)); err != nil {
		return NewError("DISPUTE_STATUS_INVALID", "dispute status is invalid")
	}
	if d.HoldID == "" {
		return NewError("DISPUTE_HOLD_REQUIRED", "dispute requires a durable hold reference")
	}
	if d.PolicyVersion == "" {
		return NewError("DISPUTE_POLICY_REQUIRED", "dispute requires a versioned network policy")
	}
	return nil
}
