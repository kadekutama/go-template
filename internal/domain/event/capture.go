package event

import (
	"errors"
	"time"
)

// PaymentCapturedPayload records a full or partial capture posting.
type PaymentCapturedPayload struct {
	PaymentID     string `json:"payment_id"`
	TenantID      string `json:"tenant_id"`
	PostingID     string `json:"posting_id"`
	CapturedMinor int64  `json:"captured_minor"`
	AssetCode     string `json:"asset_code"`
	IsPartial     bool   `json:"is_partial"`
}

// NewPaymentCaptured builds payment.captured.v1.
func NewPaymentCaptured(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentCapturedPayload, meta EventMetadata) (TypedEvent[PaymentCapturedPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[PaymentCapturedPayload]{}, err
	}
	if payload.CapturedMinor <= 0 {
		return TypedEvent[PaymentCapturedPayload]{}, errors.New("event: captured_minor must be positive")
	}
	return newTyped("payment.captured.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}

// PaymentSettledPayload records confirmed provider/bank settlement.
type PaymentSettledPayload struct {
	PaymentID         string    `json:"payment_id"`
	TenantID          string    `json:"tenant_id"`
	Provider          string    `json:"provider"`
	ProviderObjectID  string    `json:"provider_object_id"`
	SettlementBatchID string    `json:"settlement_batch_id"`
	PostingID         string    `json:"posting_id"`
	AmountMinor       int64     `json:"amount_minor"`
	AssetCode         string    `json:"asset_code"`
	ProviderTraceID   string    `json:"provider_trace_id"`
	SettledAt         time.Time `json:"settled_at"`
}

// NewPaymentSettled builds payment.settled.v1.
func NewPaymentSettled(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentSettledPayload, meta EventMetadata) (TypedEvent[PaymentSettledPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[PaymentSettledPayload]{}, err
	}
	return newTyped("payment.settled.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}
