package workflow

import (
	"context"
	"time"
)

// ReconciliationActions carries the reconciliation saga effects keyed on
// sagaID:stepName. Matching moves no money, so no step compensates.
type ReconciliationActions struct {
	MatchStatements func(ctx context.Context) error
	ResolveBreaks   func(ctx context.Context) error
}

// ReconciliationSteps builds the reconciliation saga: match → resolve.
// Breaks route to the resolution workflow; nothing compensates because
// matching moves no money.
func ReconciliationSteps(actions ReconciliationActions) []Step {
	return []Step{
		{Name: "match-statements", MaxAttempts: 2, Timeout: 60 * time.Second, Run: actions.MatchStatements},
		{Name: "resolve-breaks", MaxAttempts: 3, Timeout: 60 * time.Second, Run: actions.ResolveBreaks},
	}
}
