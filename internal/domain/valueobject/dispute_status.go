package valueobject

import (
	"fmt"
)

// DisputeStatus is the dispute workflow state.
type DisputeStatus string

// Dispute lifecycle states.
const (
	DisputeOpen        DisputeStatus = "OPEN"
	DisputeEvidenceDue DisputeStatus = "EVIDENCE_DUE"
	DisputeUnderReview DisputeStatus = "UNDER_REVIEW"
	DisputeWon         DisputeStatus = "WON"
	DisputeLost        DisputeStatus = "LOST"
	DisputeClosed      DisputeStatus = "CLOSED"
)

// ParseDisputeStatus validates a dispute state.
func ParseDisputeStatus(s string) (DisputeStatus, error) {
	switch DisputeStatus(s) {
	case DisputeOpen, DisputeEvidenceDue, DisputeUnderReview, DisputeWon, DisputeLost, DisputeClosed:
		return DisputeStatus(s), nil
	default:
		return "", fmt.Errorf("dispute: invalid status %q", s)
	}
}

// CanTransitionDispute reports legal dispute moves.
func CanTransitionDispute(from, to DisputeStatus) bool {
	switch from {
	case DisputeOpen:
		return to == DisputeEvidenceDue || to == DisputeUnderReview
	case DisputeEvidenceDue:
		return to == DisputeUnderReview || to == DisputeClosed
	case DisputeUnderReview:
		return to == DisputeWon || to == DisputeLost
	case DisputeWon, DisputeLost:
		return to == DisputeClosed
	default:
		return false
	}
}
