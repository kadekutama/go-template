// Package egress enforces per-provider outbound quotas with blocking Wait
// semantics over the existing port.RateLimiter token bucket (E09.1-T03). It
// adds no Lua script and changes no E08-T07 behavior: provider-operation keys
// and admission decisions flow through the accepted adapter as-is. E10
// provider tasks consume this capability once per attempt.
package egress

import (
	"crypto/rand"
	"math/big"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	"github.com/kadekutama/go-template/internal/shared/kernel/sleep"
)

// JitterSource spreads synchronized retries. Implementations must return a
// non-negative delay bounded by the given upper bound; callers add it only
// after the returned RetryAfter, so retries never precede it.
type JitterSource interface {
	Jitter(upper time.Duration) time.Duration
}

// CappedJitter spreads retries uniformly over [0, upper] using
// cryptographic randomness. Jitter carries no secrets, but crypto/rand
// keeps the gosec surface clean and avoids seeding concerns entirely.
type CappedJitter struct{}

// NewCappedJitter builds the production jitter source.
func NewCappedJitter() CappedJitter {
	return CappedJitter{}
}

// Jitter returns a uniform delay in [0, upper]; non-positive upper yields 0.
// A randomness failure degrades to zero jitter, which stays safe because the
// caller always waits at least RetryAfter.
func (CappedJitter) Jitter(upper time.Duration) time.Duration {
	if upper <= 0 {
		return 0
	}

	drawn, err := rand.Int(rand.Reader, big.NewInt(int64(upper)+1))
	if err != nil {
		return 0
	}

	return time.Duration(drawn.Int64())
}

// Params wires an Limiter (Parameter Object pattern). Limiter is
// required; Clock, Jitter, and Sleeper default to production implementations
// when nil. Policies may be empty only to fail every call closed with a
// missing-policy error; use at least one policy in production.
type Params struct {
	Limiter  appport.RateLimiter
	Policies []Policy
	Clock    kernel.Clock
	Jitter   JitterSource
	Sleeper  sleep.Sleeper
}
