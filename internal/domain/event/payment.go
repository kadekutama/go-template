package event

import (
	"time"
)

// PaymentIntentCreatedPayload records a created payment intent.
type PaymentIntentCreatedPayload struct {
	PaymentID   string `json:"payment_id"`
	TenantID    string `json:"tenant_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
}

// NewPaymentIntentCreated builds payment_intent.created.v1.
func NewPaymentIntentCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentIntentCreatedPayload, meta EventMetadata) (TypedEvent[PaymentIntentCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID, "asset_code": payload.AssetCode}); err != nil {
		return TypedEvent[PaymentIntentCreatedPayload]{}, err
	}
	return newTyped("payment_intent.created.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}

// PaymentSucceededPayload records processor success (triggers a ledger posting downstream).
type PaymentSucceededPayload struct {
	PaymentID   string `json:"payment_id"`
	TenantID    string `json:"tenant_id"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
}

// NewPaymentIntentSucceeded builds payment_intent.succeeded.v1.
func NewPaymentIntentSucceeded(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentSucceededPayload, meta EventMetadata) (TypedEvent[PaymentSucceededPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[PaymentSucceededPayload]{}, err
	}
	return newTyped("payment_intent.succeeded.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}

// PaymentFailedPayload records a processor decline or error.
type PaymentFailedPayload struct {
	PaymentID string `json:"payment_id"`
	TenantID  string `json:"tenant_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// NewPaymentIntentFailed builds payment_intent.failed.v1.
func NewPaymentIntentFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentFailedPayload, meta EventMetadata) (TypedEvent[PaymentFailedPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[PaymentFailedPayload]{}, err
	}
	return newTyped("payment_intent.failed.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}

// PaymentCanceledPayload records a user/API cancellation.
type PaymentCanceledPayload struct {
	PaymentID   string `json:"payment_id"`
	TenantID    string `json:"tenant_id"`
	CancelledBy string `json:"cancelled_by"`
}

// NewPaymentIntentCanceled builds payment_intent.canceled.v1 (American spelling).
func NewPaymentIntentCanceled(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentCanceledPayload, meta EventMetadata) (TypedEvent[PaymentCanceledPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[PaymentCanceledPayload]{}, err
	}
	return newTyped("payment_intent.canceled.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}

// PaymentActionRequiredPayload records an SCA/3DS challenge requirement.
type PaymentActionRequiredPayload struct {
	PaymentID  string `json:"payment_id"`
	TenantID   string `json:"tenant_id"`
	ActionType string `json:"action_type"`
	ActionURL  string `json:"action_url"`
}

// NewPaymentIntentRequiresAction builds payment_intent.requires_action.v1.
func NewPaymentIntentRequiresAction(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload PaymentActionRequiredPayload, meta EventMetadata) (TypedEvent[PaymentActionRequiredPayload], error) {
	if err := requireIDs(map[string]string{fieldPaymentID: payload.PaymentID, fieldTenantID: payload.TenantID, "action_type": payload.ActionType}); err != nil {
		return TypedEvent[PaymentActionRequiredPayload]{}, err
	}
	return newTyped("payment_intent.requires_action.v1", eventID, aggregateID, "PaymentIntent", occurredAt, version, seq, payload, meta)
}
