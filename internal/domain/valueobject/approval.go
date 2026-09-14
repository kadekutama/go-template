package valueobject

import (
	"fmt"
)

// BreakStatus is the break workflow state.
type BreakStatus string

// Break workflow states.
const (
	BreakOpen         BreakStatus = "OPEN"
	BreakInReview     BreakStatus = "IN_REVIEW"
	BreakResolved     BreakStatus = "RESOLVED"
	BreakAcknowledged BreakStatus = "ACKNOWLEDGED"
	BreakEscalated    BreakStatus = "ESCALATED"
)

// ParseBreakStatus validates a break state.
func ParseBreakStatus(s string) (BreakStatus, error) {
	switch BreakStatus(s) {
	case BreakOpen, BreakInReview, BreakResolved, BreakAcknowledged, BreakEscalated:
		return BreakStatus(s), nil
	default:
		return "", fmt.Errorf("reconciliation: invalid break status %q", s)
	}
}

// ResolutionAction is the break resolution action.
type ResolutionAction string

// Resolution actions.
const (
	ResolveAdjustLedger  ResolutionAction = "ADJUST_LEDGER"
	ResolveExternalError ResolutionAction = "MARK_EXTERNAL_ERROR"
	ResolveEscalate      ResolutionAction = "ESCALATE_TO_COMPLIANCE"
	ResolveAcknowledge   ResolutionAction = "ACKNOWLEDGE"
)

// ParseResolutionAction validates a resolution action.
func ParseResolutionAction(s string) (ResolutionAction, error) {
	switch ResolutionAction(s) {
	case ResolveAdjustLedger, ResolveExternalError, ResolveEscalate, ResolveAcknowledge:
		return ResolutionAction(s), nil
	default:
		return "", fmt.Errorf("reconciliation: invalid resolution action %q", s)
	}
}

// CanTransitionBreak reports legal break moves.
func CanTransitionBreak(from BreakStatus, action ResolutionAction) bool {
	switch from {
	case BreakOpen:
		return action == ResolveAcknowledge || action == ResolveEscalate || action == ResolveExternalError || action == ResolveAdjustLedger
	case BreakInReview:
		return action == ResolveAdjustLedger || action == ResolveExternalError || action == ResolveEscalate || action == ResolveAcknowledge
	case BreakAcknowledged:
		return false
	case BreakEscalated:
		return action == ResolveAdjustLedger || action == ResolveExternalError
	default:
		return false
	}
}
