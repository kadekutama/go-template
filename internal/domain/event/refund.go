package event

import (
	"errors"
	"time"
)

// RefundCreatedPayload records an accepted refund command.
type RefundCreatedPayload struct {
	RefundID          string `json:"refund_id"`
	TenantID          string `json:"tenant_id"`
	OriginalPostingID string `json:"original_posting_id"`
	RefundAmountMinor int64  `json:"refund_amount_minor"`
	AssetCode         string `json:"asset_code"`
}

// NewRefundCreated builds refund.created.v1.
func NewRefundCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload RefundCreatedPayload, meta EventMetadata) (TypedEvent[RefundCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldRefundID: payload.RefundID, fieldTenantID: payload.TenantID, "original_posting_id": payload.OriginalPostingID}); err != nil {
		return TypedEvent[RefundCreatedPayload]{}, err
	}
	if payload.RefundAmountMinor <= 0 {
		return TypedEvent[RefundCreatedPayload]{}, errors.New("event: refund_amount_minor must be positive")
	}
	return newTyped("refund.created.v1", eventID, aggregateID, "Refund", occurredAt, version, seq, payload, meta)
}

// RefundSucceededPayload records a processed and posted refund.
type RefundSucceededPayload struct {
	RefundID  string `json:"refund_id"`
	TenantID  string `json:"tenant_id"`
	PostingID string `json:"posting_id"`
}

// NewRefundSucceeded builds refund.succeeded.v1.
func NewRefundSucceeded(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload RefundSucceededPayload, meta EventMetadata) (TypedEvent[RefundSucceededPayload], error) {
	if err := requireIDs(map[string]string{fieldRefundID: payload.RefundID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[RefundSucceededPayload]{}, err
	}
	return newTyped("refund.succeeded.v1", eventID, aggregateID, "Refund", occurredAt, version, seq, payload, meta)
}

// RefundFailedPayload records a failed refund.
type RefundFailedPayload struct {
	RefundID string `json:"refund_id"`
	TenantID string `json:"tenant_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// NewRefundFailed builds refund.failed.v1.
func NewRefundFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload RefundFailedPayload, meta EventMetadata) (TypedEvent[RefundFailedPayload], error) {
	if err := requireIDs(map[string]string{fieldRefundID: payload.RefundID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[RefundFailedPayload]{}, err
	}
	return newTyped("refund.failed.v1", eventID, aggregateID, "Refund", occurredAt, version, seq, payload, meta)
}
