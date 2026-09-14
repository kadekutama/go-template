package valueobject

import (
	"fmt"
	"slices"
)

// PaymentStatus is the payment workflow state. It is never a posting state.
// Provider timeouts are represented by the service-level OUTCOME_UNKNOWN
// resolution (status lookup before retry), not by a workflow state.
type PaymentStatus string

// Payment lifecycle states.
const (
	PaymentRequiresMethod PaymentStatus = "REQUIRES_METHOD"
	PaymentRequiresAction PaymentStatus = "REQUIRES_ACTION"
	PaymentAuthorized     PaymentStatus = "AUTHORIZED"
	PaymentCaptured       PaymentStatus = "CAPTURED"
	PaymentPendingSettle  PaymentStatus = "PENDING_SETTLEMENT"
	PaymentSettled        PaymentStatus = "SETTLED"
	PaymentFailed         PaymentStatus = "FAILED"
	PaymentReturned       PaymentStatus = "RETURNED"
)

var validPaymentTransitions = map[PaymentStatus][]PaymentStatus{
	PaymentRequiresMethod: {PaymentRequiresAction, PaymentAuthorized, PaymentFailed},
	PaymentRequiresAction: {PaymentAuthorized, PaymentFailed},
	PaymentAuthorized:     {PaymentCaptured, PaymentFailed},
	PaymentCaptured:       {PaymentPendingSettle, PaymentFailed},
	PaymentPendingSettle:  {PaymentSettled, PaymentFailed, PaymentReturned},
}

// ParsePaymentStatus validates a payment state.
func ParsePaymentStatus(s string) (PaymentStatus, error) {
	switch PaymentStatus(s) {
	case PaymentRequiresMethod, PaymentRequiresAction, PaymentAuthorized,
		PaymentCaptured, PaymentPendingSettle, PaymentSettled,
		PaymentFailed, PaymentReturned:
		return PaymentStatus(s), nil
	default:
		return "", fmt.Errorf("payment: invalid status %q", s)
	}
}

// CanTransitionPayment reports legal workflow moves.
func CanTransitionPayment(from, to PaymentStatus) bool {
	targets, ok := validPaymentTransitions[from]
	if !ok {
		return false
	}
	return slices.Contains(targets, to)
}
