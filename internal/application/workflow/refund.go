package workflow

import (
	"context"
	"time"
)

// RefundActions carries the refund saga effects keyed on sagaID:stepName.
type RefundActions struct {
	ValidateWindow func(ctx context.Context) error
	ExecuteRefund  func(ctx context.Context) error
	LinkReversal   func(ctx context.Context) error
	Notify         func(ctx context.Context) error
}

// RefundSteps builds the refund saga: validate → execute → notify. A failed
// execute links the reversal as compensation; validation and notification
// carry nothing to undo.
func RefundSteps(actions RefundActions) []Step {
	return []Step{
		{Name: "validate-window", MaxAttempts: 1, Timeout: 10 * time.Second, Run: actions.ValidateWindow},
		{Name: "execute-refund", MaxAttempts: 3, Timeout: 30 * time.Second, Run: actions.ExecuteRefund, Compensate: actions.LinkReversal},
		{Name: StepNotify, MaxAttempts: 5, Timeout: 10 * time.Second, Run: actions.Notify},
	}
}
