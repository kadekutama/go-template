package valueobject

import (
	"fmt"
)

// TopupStatus is the top-up workflow state. It is never a posting state.
type TopupStatus string

// Top-up lifecycle states.
const (
	TopupPending   TopupStatus = "PENDING"
	TopupSucceeded TopupStatus = "SUCCEEDED"
	TopupFailed    TopupStatus = "FAILED"
	TopupCanceled  TopupStatus = "CANCELED"
)

// ParseTopupStatus validates a top-up state.
func ParseTopupStatus(s string) (TopupStatus, error) {
	switch TopupStatus(s) {
	case TopupPending, TopupSucceeded, TopupFailed, TopupCanceled:
		return TopupStatus(s), nil
	default:
		return "", fmt.Errorf("topup: invalid status %q", s)
	}
}

// CanTransitionTopup reports legal top-up moves: PENDING resolves once.
func CanTransitionTopup(from, to TopupStatus) bool {
	return from == TopupPending && (to == TopupSucceeded || to == TopupFailed || to == TopupCanceled)
}
