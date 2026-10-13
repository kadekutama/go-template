package resilience

import (
	"time"
)

// RetryPolicy bounds retries. All parameters must be explicit and validated.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// Validate ensures all policy parameters are explicitly positive and valid.
func (p RetryPolicy) Validate() error {
	if p.MaxAttempts <= 0 {
		return ErrResilienceMaxAttemptsMustBePositive
	}

	if p.InitialBackoff <= 0 {
		return ErrResilienceInitialBackoffMustBePositive
	}

	if p.MaxBackoff < p.InitialBackoff {
		return ErrResilienceMaxBackoffMustBeGreaterThanOrEqualToInitialBack
	}

	if p.Multiplier < 1.0 {
		return ErrResilienceMultiplierMustBeAtLeast10
	}

	return nil
}

// BackoffFor returns the wait before attempt number (1-based first retry).
// Growth is exponential by Multiplier, capped at MaxBackoff.
func (p RetryPolicy) BackoffFor(retryNumber int) (time.Duration, error) {
	if err := p.Validate(); err != nil {
		return 0, err
	}

	if retryNumber <= 0 {
		retryNumber = 1
	}

	wait := float64(p.InitialBackoff)
	for i := 1; i < retryNumber; i++ {
		wait *= p.Multiplier
		if wait >= float64(p.MaxBackoff) {
			return p.MaxBackoff, nil
		}
	}

	if wait >= float64(p.MaxBackoff) {
		return p.MaxBackoff, nil
	}

	return time.Duration(wait), nil
}

// ShouldRetry encodes the no-blind-retry rule: Permanent never retries;
// anything else retries ONLY when the call is idempotent or carries a
// durable/provider idempotency key.
func ShouldRetry(class Class, idempotent bool, idempotencyKey string) bool {
	if class == Permanent {
		return false
	}

	return idempotent || idempotencyKey != ""
}
