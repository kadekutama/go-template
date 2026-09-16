package workflow

import (
	"context"
	"time"
)

// PayoutSettlementActions carries the payout-settlement saga effects keyed on
// sagaID:stepName.
type PayoutSettlementActions struct {
	SubmitPayout   func(ctx context.Context) error
	HoldForFailure func(ctx context.Context) error
	SettlePayout   func(ctx context.Context) error
	MarkUnsettled  func(ctx context.Context) error
	Notify         func(ctx context.Context) error
}

// PayoutSettlementSteps builds the payout-settlement saga: submit → settle →
// notify. A settlement failure holds funds and notifies — it never writes
// partial ledger state.
func PayoutSettlementSteps(actions PayoutSettlementActions) []Step {
	return []Step{
		{Name: "submit-payout", MaxAttempts: 3, Timeout: 10 * time.Second, Run: actions.SubmitPayout, Compensate: actions.HoldForFailure},
		{Name: "settle-payout", MaxAttempts: 5, Timeout: 30 * time.Second, Run: actions.SettlePayout, Compensate: actions.MarkUnsettled},
		{Name: StepNotify, MaxAttempts: 5, Timeout: 10 * time.Second, Run: actions.Notify},
	}
}
