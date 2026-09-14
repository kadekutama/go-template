package resilience

import "time"

// Defaults keep zero-value policies safe: one attempt, fixed short backoff.
const (
	DefaultMaxAttempts    = 1
	DefaultInitialBackoff = 100 * time.Millisecond
	DefaultMaxBackoff     = 5 * time.Second
	DefaultMultiplier     = 2.0
)

// RetryPolicy bounds retries. Use Normalize before executing.
type RetryPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
}

// Normalize fills zero values with defaults.
func (p RetryPolicy) Normalize() RetryPolicy {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = DefaultMaxAttempts
	}
	if p.InitialBackoff <= 0 {
		p.InitialBackoff = DefaultInitialBackoff
	}
	if p.MaxBackoff <= 0 {
		p.MaxBackoff = DefaultMaxBackoff
	}
	if p.Multiplier <= 0 {
		p.Multiplier = DefaultMultiplier
	}
	return p
}

// BackoffFor returns the wait before attempt number (1-based first retry).
// Growth is exponential by Multiplier, capped at MaxBackoff.
func (p RetryPolicy) BackoffFor(retryNumber int) time.Duration {
	p = p.Normalize()
	if retryNumber <= 0 {
		retryNumber = 1
	}
	wait := float64(p.InitialBackoff)
	for i := 1; i < retryNumber; i++ {
		wait *= p.Multiplier
		if wait >= float64(p.MaxBackoff) {
			return p.MaxBackoff
		}
	}
	if wait >= float64(p.MaxBackoff) {
		return p.MaxBackoff
	}
	return time.Duration(wait)
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
