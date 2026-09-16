package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PaymentIntentRequest starts a charge against a payment method. Confirmation
// delegates charging to the payment-processor port; ledger entries post on
// the success event. SCA challenges surface requires_action for the edge.
type PaymentIntentRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	AmountMinor    int64
	AssetCode      valueobject.AssetCode
	Method         valueobject.PaymentMethod
	IdempotencyKey string
	Actor          string
}

// PaymentIntentResult tracks the intent through its workflow states.
type PaymentIntentResult struct {
	IntentID string
	Status   string
	Cursor   string
}

// ConfirmIntentRequest captures a previously authorized intent, in full or
// partially per network rules (E03-T08 capture validation).
type ConfirmIntentRequest struct {
	TenantID       valueobject.TenantID
	IntentID       string
	CaptureMinor   int64
	IdempotencyKey string
	Actor          string
}

// RefundRequest reverses a captured payment in full or partially, inside the
// configurable window with reversal linkage to the original posting.
type RefundRequest struct {
	TenantID       valueobject.TenantID
	OriginalTxn    string
	AmountMinor    int64
	Reason         string
	IdempotencyKey string
	Actor          string
}

// RefundResult links the reversal to the original payment.
type RefundResult struct {
	RefundID   string
	OriginalID string
	Status     string
	Cursor     string
}

// PayoutRequest stages a two-stage payout honoring the E03-T11 eligibility
// policy: ineligible requests fail with PAYOUT_BLOCKED reason details and
// never reach the provider. DestinationVerified and TenantAgeDays feed the
// eligibility input (added in E06-T04).
type PayoutRequest struct {
	TenantID            valueobject.TenantID
	LedgerID            valueobject.LedgerID
	AccountID           valueobject.AccountID
	AmountMinor         int64
	AssetCode           valueobject.AssetCode
	Method              valueobject.PayoutMethod
	DestinationVerified bool
	TenantAgeDays       int
	IdempotencyKey      string
	Actor               string
}

// PayoutResult tracks submission through settlement.
type PayoutResult struct {
	PayoutID string
	Status   string
	Cursor   string
}

// TopupRequest funds platform/merchant balance from a verified external bank
// instrument (never a ledger account).
type TopupRequest struct {
	TenantID           valueobject.TenantID
	LedgerID           valueobject.LedgerID
	CreditAccount      valueobject.AccountID
	AmountMinor        int64
	AssetCode          valueobject.AssetCode
	InstrumentID       string
	InstrumentVerified bool
	IdempotencyKey     string
	Actor              string
}

// TopupResult tracks the top-up through settlement.
type TopupResult struct {
	TopupID string
	Status  string
	Cursor  string
}

// PaymentQuery reads one payment object by tenant + ID. Strong read. Actor
// carries the canceling identity for CancelIntent/CancelPayout; plain reads
// ignore it.
type PaymentQuery struct {
	TenantID string
	ID       string
	Actor    string
}

// PaymentCommandUseCases defines the mutating operations on payments, refunds, payouts, and top-ups. Strong writes.
type PaymentCommandUseCases interface {
	// CreateIntent creates a payment intent in REQUIRES_METHOD. Strong write.
	CreateIntent(ctx context.Context, req PaymentIntentRequest) (PaymentIntentResult, error)
	// ConfirmIntent confirms and captures an intent. Strong write.
	ConfirmIntent(ctx context.Context, req ConfirmIntentRequest) (PaymentIntentResult, error)
	// CancelIntent cancels an uncaptured intent. Strong write.
	CancelIntent(ctx context.Context, query PaymentQuery) (PaymentIntentResult, error)
	// CreateRefund validates window/amount and links the reversal. Strong write.
	CreateRefund(ctx context.Context, req RefundRequest) (RefundResult, error)
	// CreatePayout gates on eligibility then stages settlement. Strong write.
	CreatePayout(ctx context.Context, req PayoutRequest) (PayoutResult, error)
	// CancelPayout cancels a PENDING payout. Strong write.
	CancelPayout(ctx context.Context, query PaymentQuery) (PayoutResult, error)
	// CreateTopup validates the instrument and stages funding. Strong write.
	CreateTopup(ctx context.Context, req TopupRequest) (TopupResult, error)
}

// PaymentQueryUseCases defines the read operations on payments, refunds, payouts, and top-ups. Strong reads.
type PaymentQueryUseCases interface {
	// GetIntent returns one payment intent. Strong read.
	GetIntent(ctx context.Context, query PaymentQuery) (PaymentIntentResult, error)
	// ListIntents returns one tenant's intents, newest first (bounded). Strong read.
	ListIntents(ctx context.Context, tenant valueobject.TenantID, limit int) ([]PaymentIntentResult, error)
	// GetRefund returns one refund. Strong read.
	GetRefund(ctx context.Context, query PaymentQuery) (RefundResult, error)
	// ListRefunds returns one tenant's refunds, newest first (bounded). Strong read.
	ListRefunds(ctx context.Context, tenant valueobject.TenantID, limit int) ([]RefundResult, error)
	// GetPayout returns one payout. Strong read.
	GetPayout(ctx context.Context, query PaymentQuery) (PayoutResult, error)
	// ListPayouts returns one tenant's payouts, newest first (bounded). Strong read.
	ListPayouts(ctx context.Context, tenant valueobject.TenantID, limit int) ([]PayoutResult, error)
	// GetTopup returns one top-up. Strong read.
	GetTopup(ctx context.Context, tenant valueobject.TenantID, topupID string) (TopupResult, error)
	// ListTopups returns one tenant's top-ups, newest first (bounded). Strong read.
	ListTopups(ctx context.Context, tenant valueobject.TenantID, limit int) ([]TopupResult, error)
}

// PaymentUseCases is the composite inbound payment/refund/payout/topup surface
// (implemented in E06-T04).
type PaymentUseCases interface {
	PaymentCommandUseCases
	PaymentQueryUseCases
}
