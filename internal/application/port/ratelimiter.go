package port

import (
	"context"
	"time"
)

// RateLimitDecision answers one admission check with the caller's remaining
// budget for observability.
type RateLimitDecision struct {
	// Allowed is false when the key exhausted its budget in Window.
	Allowed bool
	// Remaining carries the budget left after this check.
	Remaining int64
	// RetryAfter hints when budget may be available again.
	RetryAfter time.Duration
}

// RateLimiter is the admission boundary (Valkey-backed in E08, deterministic
// test adapter elsewhere). Limits are per key + window; denial is a decision,
// never an error, so callers map it to 429 at the edge. State is best-effort:
// under store failure callers fail open for reads and fail closed for money
// movement, documented at the call site.
type RateLimiter interface {
	// Allow checks one key against budget per window. Best-effort read.
	Allow(ctx context.Context, key string, budget int64, window time.Duration) (RateLimitDecision, error)
}
