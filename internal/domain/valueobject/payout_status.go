package valueobject

import (
	"fmt"
)

// PayoutStatus is the payout workflow state. It is never a posting state.
type PayoutStatus string

// Payout lifecycle states.
const (
	PayoutPending   PayoutStatus = "PENDING"
	PayoutInTransit PayoutStatus = "IN_TRANSIT"
	PayoutPaid      PayoutStatus = "PAID"
	PayoutFailed    PayoutStatus = "FAILED"
	PayoutCanceled  PayoutStatus = "CANCELED"
)

// ParsePayoutStatus validates a payout state.
func ParsePayoutStatus(s string) (PayoutStatus, error) {
	switch PayoutStatus(s) {
	case PayoutPending, PayoutInTransit, PayoutPaid, PayoutFailed, PayoutCanceled:
		return PayoutStatus(s), nil
	default:
		return "", fmt.Errorf("payout: invalid status %q", s)
	}
}

// CanTransition reports legal workflow moves: PENDING → IN_TRANSIT /
// CANCELED, IN_TRANSIT → PAID / FAILED.
func CanTransition(from, to PayoutStatus) bool {
	switch from {
	case PayoutPending:
		return to == PayoutInTransit || to == PayoutCanceled
	case PayoutInTransit:
		return to == PayoutPaid || to == PayoutFailed
	default:
		return false
	}
}

// CanCancel reports whether a payout may be canceled (PENDING only).
func CanCancel(s PayoutStatus) bool {
	return s == PayoutPending
}
