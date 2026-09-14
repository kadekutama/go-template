package valueobject

import (
	"fmt"
)

// RecoveryStatus is the durable recovery workflow state.
type RecoveryStatus string

// Recovery states.
const (
	RecoveryPending   RecoveryStatus = "PENDING"
	RecoveryCollected RecoveryStatus = "COLLECTED"
	RecoveryFailed    RecoveryStatus = "FAILED"
	RecoveryCanceled  RecoveryStatus = "CANCELED"
)

// ParseRecoveryStatus validates a recovery state.
func ParseRecoveryStatus(s string) (RecoveryStatus, error) {
	switch RecoveryStatus(s) {
	case RecoveryPending, RecoveryCollected, RecoveryFailed, RecoveryCanceled:
		return RecoveryStatus(s), nil
	default:
		return "", fmt.Errorf("recovery: invalid status %q", s)
	}
}

// CanTransitionRecovery reports legal recovery moves: PENDING resolves once
// (collected, failed, or explicitly canceled).
func CanTransitionRecovery(from, to RecoveryStatus) bool {
	return from == RecoveryPending && (to == RecoveryCollected || to == RecoveryFailed || to == RecoveryCanceled)
}
