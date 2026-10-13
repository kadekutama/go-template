package workflow

import (
	"context"
	"time"
)

// BatchActions carries the batch saga effects keyed on sagaID:stepName. Item
// independence is structural: item failures record per-item outcomes and
// never roll back siblings, so no step compensates.
type BatchActions struct {
	IntakeBatch  func(ctx context.Context) error
	ExecuteItems func(ctx context.Context) error
	CloseBatch   func(ctx context.Context) error
}

// BatchSteps builds the batch saga: intake → execute → close. Partial item
// outcomes close PARTIAL through the normal path, not through compensation.
func BatchSteps(actions BatchActions) []Step {
	return []Step{
		{Name: "intake-batch", MaxAttempts: maxAttempts2, Timeout: timeout10 * time.Second, Run: actions.IntakeBatch},
		{Name: "execute-items", MaxAttempts: 1, Timeout: timeout5 * time.Minute, Run: actions.ExecuteItems},
		{Name: "close-batch", MaxAttempts: maxAttempts3, Timeout: timeout10 * time.Second, Run: actions.CloseBatch},
	}
}
