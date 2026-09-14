package event

import (
	"errors"
	"time"
)

// PayoutCreatedPayload records an accepted payout command.
type PayoutCreatedPayload struct {
	PayoutID    string `json:"payout_id"`
	TenantID    string `json:"tenant_id"`
	AccountID   string `json:"account_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
	Method      string `json:"method"`
}

// NewPayoutCreated builds payout.created.v1.
func NewPayoutCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PayoutCreatedPayload, meta EventMetadata) (TypedEvent[PayoutCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldPayoutID: payload.PayoutID, fieldTenantID: payload.TenantID, fieldAccountID: payload.AccountID}); err != nil {
		return TypedEvent[PayoutCreatedPayload]{}, err
	}
	if payload.AmountMinor <= 0 {
		return TypedEvent[PayoutCreatedPayload]{}, errors.New("event: amount_minor must be positive")
	}
	return newTyped("payout.created.v1", eventID, aggregateID, "Payout", occurredAt, version, seq, payload, meta)
}

// PayoutPendingPayload records submission to the payout network.
type PayoutPendingPayload struct {
	PayoutID   string `json:"payout_id"`
	TenantID   string `json:"tenant_id"`
	ProviderID string `json:"provider_id"`
}

// NewPayoutPending builds payout.pending.v1.
func NewPayoutPending(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PayoutPendingPayload, meta EventMetadata) (TypedEvent[PayoutPendingPayload], error) {
	if err := requireIDs(map[string]string{fieldPayoutID: payload.PayoutID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[PayoutPendingPayload]{}, err
	}
	return newTyped("payout.pending.v1", eventID, aggregateID, "Payout", occurredAt, version, seq, payload, meta)
}

// PayoutPaidPayload records bank-confirmed settlement.
type PayoutPaidPayload struct {
	PayoutID  string `json:"payout_id"`
	TenantID  string `json:"tenant_id"`
	PostingID string `json:"posting_id"`
}

// NewPayoutPaid builds payout.paid.v1.
func NewPayoutPaid(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PayoutPaidPayload, meta EventMetadata) (TypedEvent[PayoutPaidPayload], error) {
	if err := requireIDs(map[string]string{fieldPayoutID: payload.PayoutID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[PayoutPaidPayload]{}, err
	}
	return newTyped("payout.paid.v1", eventID, aggregateID, "Payout", occurredAt, version, seq, payload, meta)
}

// PayoutFailedPayload records a network reject or error.
type PayoutFailedPayload struct {
	PayoutID string `json:"payout_id"`
	TenantID string `json:"tenant_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// NewPayoutFailed builds payout.failed.v1.
func NewPayoutFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PayoutFailedPayload, meta EventMetadata) (TypedEvent[PayoutFailedPayload], error) {
	if err := requireIDs(map[string]string{fieldPayoutID: payload.PayoutID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[PayoutFailedPayload]{}, err
	}
	return newTyped("payout.failed.v1", eventID, aggregateID, "Payout", occurredAt, version, seq, payload, meta)
}
