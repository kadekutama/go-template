package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ChargeRequest asks the provider to move money. The idempotency key is
// provider-scoped: retries reuse it, and provider timeouts resolve through
// GetStatus before any retry that could move money twice. IntentID scopes
// provider-side dedupe to the intent: concurrent confirms of one intent
// collapse server-side even across distinct request keys (added in audit;
// E10 honors it alongside the key).
type ChargeRequest struct {
	TenantID       valueobject.TenantID
	IntentID       string
	AmountMinor    int64
	AssetCode      valueobject.AssetCode
	Method         valueobject.PaymentMethod
	InstrumentRef  string
	IdempotencyKey string
	Timeout        time.Duration
}

// ChargeResult is the provider's charge answer, including challenge state.
type ChargeResult struct {
	ProviderID     string
	Status         string
	RequiresAction bool
	TraceID        string
}

// RefundChargeRequest reverses a provider charge under explicit policy.
type RefundChargeRequest struct {
	TenantID       valueobject.TenantID
	ProviderID     string
	AmountMinor    int64
	IdempotencyKey string
	Timeout        time.Duration
}

// ChallengeCompletion carries the SCA/3DS challenge result back to the
// provider (maps to payment_intent.requires_action resolution).
type ChallengeCompletion struct {
	TenantID   valueobject.TenantID
	ProviderID string
	Succeeded  bool
	Timeout    time.Duration
}

// PaymentProcessor is the charging boundary (implemented in E10 with a live
// adapter and a zero-credential sandbox fake). Timeouts are per call via
// Timeout or ctx deadline, whichever is tighter; retry is caller-owned and
// MUST reuse the idempotency key; provider timeouts become OUTCOME_UNKNOWN
// resolved through GetStatus, never blind retried.
type PaymentProcessor interface {
	// Charge moves money once per idempotency key. At-least-once transport;
	// exactly-once effect per key.
	Charge(ctx context.Context, req ChargeRequest) (ChargeResult, error)
	// RefundCharge reverses a charge under explicit policy. Idempotent per key.
	RefundCharge(ctx context.Context, req RefundChargeRequest) (ChargeResult, error)
	// GetStatus resolves an unknown provider outcome by idempotency key.
	// Strong read of provider state; safe to poll with backoff.
	GetStatus(ctx context.Context, tenant valueobject.TenantID, idempotencyKey string, timeout time.Duration) (ChargeResult, error)
	// CompleteChallenge finishes an SCA/3DS challenge. Idempotent per provider ID.
	CompleteChallenge(ctx context.Context, completion ChallengeCompletion) (ChargeResult, error)
}
