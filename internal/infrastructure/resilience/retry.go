package resilience

import (
	"context"
	"fmt"
	"time"

	kernel "example.com/go-template/internal/shared/kernel/resilience"
)

// SleepFunc waits d or aborts on ctx cancellation. Production passes a real
// timer; tests pass a fake advancing a fixed clock (no wall-clock waits).
type SleepFunc func(ctx context.Context, d time.Duration) error

// RealSleep is the production SleepFunc.
func RealSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Execute runs fn under policy with classified, key-gated retries. Cancellation
// (before the call or during backoff) aborts immediately without further
// attempts. The returned error wraps the last failure; exhaustion is explicit.
func Execute(
	ctx context.Context,
	sleep SleepFunc,
	policy kernel.RetryPolicy,
	idempotent bool,
	idempotencyKey string,
	classify kernel.Classifier,
	fn func(ctx context.Context) error,
) error {
	policy = policy.Normalize()
	if classify == nil {
		classify = kernel.DefaultClassifier
	}

	var lastErr error
	for attempt := 1; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = fn(ctx)
		if lastErr == nil {
			return nil
		}
		if attempt >= policy.MaxAttempts {
			break
		}
		if !kernel.ShouldRetry(classify(lastErr), idempotent, idempotencyKey) {
			break
		}
		if err := sleep(ctx, policy.BackoffFor(attempt)); err != nil {
			return err
		}
	}
	return fmt.Errorf("resilience: attempts exhausted: %w", lastErr)
}
