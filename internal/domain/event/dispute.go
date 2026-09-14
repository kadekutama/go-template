package event

import (
	"errors"
	"time"
)

// DisputeOpenedPayload records a network dispute notification or manual open.
type DisputeOpenedPayload struct {
	DisputeID     string `json:"dispute_id"`
	TenantID      string `json:"tenant_id"`
	TransactionID string `json:"transaction_id"`
	Network       string `json:"network"`
	AmountMinor   int64  `json:"amount_minor"`
	FeeMinor      int64  `json:"fee_minor"`
	AssetCode     string `json:"asset_code"`
	EvidenceDueAt string `json:"evidence_due_at"`
}

// NewDisputeOpened builds dispute.opened.v1.
func NewDisputeOpened(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload DisputeOpenedPayload, meta EventMetadata) (TypedEvent[DisputeOpenedPayload], error) {
	if err := requireIDs(map[string]string{"dispute_id": payload.DisputeID, fieldTenantID: payload.TenantID, "transaction_id": payload.TransactionID, "network": payload.Network}); err != nil {
		return TypedEvent[DisputeOpenedPayload]{}, err
	}
	if payload.AmountMinor <= 0 {
		return TypedEvent[DisputeOpenedPayload]{}, errors.New("event: amount_minor must be positive")
	}
	return newTyped("dispute.opened.v1", eventID, aggregateID, "Dispute", occurredAt, version, seq, payload, meta)
}

// DisputeClosedPayload records a won/lost decision. ReversalTxnID is set on
// LOST (the reversal is a new posting; the original is unchanged).
type DisputeClosedPayload struct {
	DisputeID     string `json:"dispute_id"`
	TenantID      string `json:"tenant_id"`
	Outcome       string `json:"outcome"`
	ReversalTxnID string `json:"reversal_txn_id,omitempty"`
}

// NewDisputeClosed builds dispute.closed.v1.
func NewDisputeClosed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload DisputeClosedPayload, meta EventMetadata) (TypedEvent[DisputeClosedPayload], error) {
	if err := requireIDs(map[string]string{"dispute_id": payload.DisputeID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[DisputeClosedPayload]{}, err
	}
	if payload.Outcome != "WON" && payload.Outcome != "LOST" {
		return TypedEvent[DisputeClosedPayload]{}, errors.New("event: outcome must be WON or LOST")
	}
	return newTyped("dispute.closed.v1", eventID, aggregateID, "Dispute", occurredAt, version, seq, payload, meta)
}
