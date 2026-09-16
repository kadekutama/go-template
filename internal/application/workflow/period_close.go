package workflow

import (
	"context"
	"time"
)

// PeriodCloseActions carries the period-close saga effects keyed on
// sagaID:stepName. Validation is read-only; the close itself is one atomic
// gate-checked operation, so neither step compensates.
type PeriodCloseActions struct {
	ValidateGates func(ctx context.Context) error
	ClosePeriod   func(ctx context.Context) error
}

// PeriodCloseSteps builds the period-close saga: validate → close. A gate
// failure aborts before close; a close failure past attempts parks the saga
// for operators instead of half-closing.
func PeriodCloseSteps(actions PeriodCloseActions) []Step {
	return []Step{
		{Name: "validate-gates", MaxAttempts: 1, Timeout: 30 * time.Second, Run: actions.ValidateGates},
		{Name: "close-period", MaxAttempts: 2, Timeout: 30 * time.Second, Run: actions.ClosePeriod},
	}
}
