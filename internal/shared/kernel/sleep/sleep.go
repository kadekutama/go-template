// Package sleep provides a context-aware suspension primitive shared by
// waiters across the codebase. Production waits use TimerSleeper; tests
// inject scripted implementations through the Sleeper interface.
package sleep

import (
	"context"
	"time"
)

// Sleeper suspends a waiter for a duration or until context cancellation,
// whichever comes first.
type Sleeper interface {
	Sleep(ctx context.Context, d time.Duration) error
}

// TimerSleeper is the production Sleeper.
type TimerSleeper struct{}

// Sleep waits for d or context cancellation, whichever comes first.
// Non-positive durations resolve the context state immediately.
func (TimerSleeper) Sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
