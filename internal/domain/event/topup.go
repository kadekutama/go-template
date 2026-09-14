package event

import (
	"errors"
	"time"
)

// TopUpPayload records a settled bank debit funding a ledger account.
type TopUpPayload struct {
	TopUpID       string `json:"topup_id"`
	TenantID      string `json:"tenant_id"`
	AccountID     string `json:"account_id"`
	AmountMinor   int64  `json:"amount_minor"`
	AssetCode     string `json:"asset_code"`
	BankAccountID string `json:"bank_account_id"`
}

// NewTopUpSucceeded builds topup.succeeded.v1.
func NewTopUpSucceeded(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TopUpPayload, meta EventMetadata) (TypedEvent[TopUpPayload], error) {
	if err := requireIDs(map[string]string{"topup_id": payload.TopUpID, fieldTenantID: payload.TenantID, fieldAccountID: payload.AccountID}); err != nil {
		return TypedEvent[TopUpPayload]{}, err
	}
	if payload.AmountMinor <= 0 {
		return TypedEvent[TopUpPayload]{}, errors.New("event: amount_minor must be positive")
	}
	return newTyped("topup.succeeded.v1", eventID, aggregateID, "TopUp", occurredAt, version, seq, payload, meta)
}

// TopUpFailedPayload records a bank reject or error.
type TopUpFailedPayload struct {
	TopUpID  string `json:"topup_id"`
	TenantID string `json:"tenant_id"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// NewTopUpFailed builds topup.failed.v1.
func NewTopUpFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TopUpFailedPayload, meta EventMetadata) (TypedEvent[TopUpFailedPayload], error) {
	if err := requireIDs(map[string]string{"topup_id": payload.TopUpID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[TopUpFailedPayload]{}, err
	}
	return newTyped("topup.failed.v1", eventID, aggregateID, "TopUp", occurredAt, version, seq, payload, meta)
}
