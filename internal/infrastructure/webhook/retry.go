package webhook

import (
	"fmt"
	"time"
)

// RetryPolicy is the immutable, config-driven retry contract: one backoff
// step per retry after the initial delivery, then DLQ. Build one with
// NewRetryPolicy (wiring maps config.WebhookConfig onto it); a policy must
// not be mutated after construction. There is intentionally no package-level
// default schedule: the backoff table lives in configuration
// (config.yaml `webhook.retry_steps_sec`, api-contracts §11), so no Go file
// hardcodes it.
type RetryPolicy struct {
	steps []time.Duration
}

// NewRetryPolicy validates and copies the backoff steps (at least one
// positive duration), so a bad configuration fails at wiring time.
func NewRetryPolicy(steps []time.Duration) (*RetryPolicy, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("webhook: at least one retry step is required")
	}

	out := make([]time.Duration, len(steps))
	copy(out, steps)

	for _, step := range out {
		if step <= 0 {
			return nil, fmt.Errorf("webhook: retry steps must be positive")
		}
	}

	return &RetryPolicy{steps: out}, nil
}

// MaxRetries reports the number of retries after the initial delivery.
func (p *RetryPolicy) MaxRetries() int {
	if p == nil {
		return 0
	}

	return len(p.steps)
}

// MaxAttempts reports the attempt bound including the initial delivery.
func (p *RetryPolicy) MaxAttempts() int {
	return p.MaxRetries() + 1
}

// Horizon reports the time from the initial delivery to the last scheduled
// retry.
func (p *RetryPolicy) Horizon() time.Duration {
	if p == nil {
		return 0
	}

	var total time.Duration
	for _, delay := range p.steps {
		total += delay
	}

	return total
}

// Schedule returns a copy of the retry backoff table.
func (p *RetryPolicy) Schedule() []time.Duration {
	if p == nil {
		return nil
	}

	out := make([]time.Duration, len(p.steps))
	copy(out, p.steps)

	return out
}

// NextDelay returns the backoff before retry attempt (0-based). ok=false
// means the schedule is exhausted and the message routes to DLQ.
func (p *RetryPolicy) NextDelay(attempt int) (delay time.Duration, ok bool) {
	if p == nil || attempt < 0 || attempt >= len(p.steps) {
		return 0, false
	}

	return p.steps[attempt], true
}
