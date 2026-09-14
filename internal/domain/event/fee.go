package event

import (
	"errors"
	"time"
)

// FeeAssessedPayload records a calculated fee.
type FeeAssessedPayload struct {
	FeeID       string `json:"fee_id"`
	TenantID    string `json:"tenant_id"`
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
	FeeType     string `json:"fee_type"`
}

// NewFeeAssessed builds fee.assessed.v1.
func NewFeeAssessed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload FeeAssessedPayload, meta EventMetadata) (TypedEvent[FeeAssessedPayload], error) {
	if err := requireIDs(map[string]string{"fee_id": payload.FeeID, fieldTenantID: payload.TenantID, fieldAccountID: payload.AccountID}); err != nil {
		return TypedEvent[FeeAssessedPayload]{}, err
	}
	if payload.AmountMinor <= 0 {
		return TypedEvent[FeeAssessedPayload]{}, errors.New("event: amount_minor must be positive")
	}
	return newTyped("fee.assessed.v1", eventID, aggregateID, "Fee", occurredAt, version, seq, payload, meta)
}

// FeeCollectedPayload records a fee posted to the ledger.
type FeeCollectedPayload struct {
	FeeID     string `json:"fee_id"`
	TenantID  string `json:"tenant_id"`
	PostingID string `json:"posting_id"`
}

// NewFeeCollected builds fee.collected.v1.
func NewFeeCollected(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload FeeCollectedPayload, meta EventMetadata) (TypedEvent[FeeCollectedPayload], error) {
	if err := requireIDs(map[string]string{"fee_id": payload.FeeID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[FeeCollectedPayload]{}, err
	}
	return newTyped("fee.collected.v1", eventID, aggregateID, "Fee", occurredAt, version, seq, payload, meta)
}
