package egress

import (
	"context"
	"errors"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	domainservice "github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	"github.com/kadekutama/go-template/internal/shared/kernel/sleep"
)

// Typed error codes for egress admission failures. All are terminal for the
// current provider operation: callers must not retry them as provider
// errors and must never count them as provider-breaker failures.
const (
	// CodeInvalidPolicy marks missing or malformed quotas (fail closed).
	CodeInvalidPolicy = "EGRESS_INVALID_POLICY"
	// CodeWaitExceeded marks max-wait exhaustion without a permit.
	CodeWaitExceeded = "EGRESS_WAIT_EXCEEDED"
	// CodeMalformedDecision marks denied decisions without a positive
	// RetryAfter (guards against tight retry loops).
	CodeMalformedDecision = "EGRESS_MALFORMED_DECISION"
	// CodeStoreUnavailable marks limiter store failures (fail closed).
	CodeStoreUnavailable = "EGRESS_STORE_UNAVAILABLE"
	// CodeContextCanceled marks caller or deadline cancellation.
	CodeContextCanceled = "EGRESS_CONTEXT_CANCELED"
)

// Error is intentionally not defined here: all simple failures use the
// shared domain error (entity.Error); context cancellation additionally
// joins the context error so errors.Is keeps working.

// policyKey scopes one quota.
type policyKey struct {
	provider  string
	operation string
}

// Limiter admits provider operations against per-operation quotas
// with blocking Wait semantics. Permits come from the shared E08-T07 token
// bucket through port.RateLimiter; this type adds keying, validation,
// bounded waiting with positive jitter, and fail-closed errors. It is safe
// for concurrent use when the wrapped limiter is.
type Limiter struct {
	limiter  appport.RateLimiter
	policies map[policyKey]Policy
	clock    kernel.Clock
	jitter   JitterSource
	sleeper  sleep.Sleeper
}

// NewEgressLimiter validates the wiring and quotas. An empty policy set is
// rejected: every provider operation needs an explicit quota, and a missing
// policy at call time fails closed rather than behaving as unlimited.
func NewEgressLimiter(params Params) (*Limiter, error) {
	if params.Limiter == nil {
		return nil, entity.NewError(CodeInvalidPolicy, "egress: rate limiter is required")
	}

	if len(params.Policies) == 0 {
		return nil, entity.NewError(CodeInvalidPolicy, "egress: at least one policy is required")
	}

	clock := params.Clock
	if clock == nil {
		clock = kernel.SystemClock{}
	}

	jitter := params.Jitter
	if jitter == nil {
		jitter = NewCappedJitter()
	}

	sleeper := params.Sleeper
	if sleeper == nil {
		sleeper = sleep.TimerSleeper{}
	}

	policies := make(map[policyKey]Policy, len(params.Policies))
	for _, policy := range params.Policies {
		if err := policy.Validate(); err != nil {
			return nil, err
		}

		key := policyKey{provider: policy.Provider, operation: policy.Operation}
		if _, exists := policies[key]; exists {
			return nil, entity.NewError(CodeInvalidPolicy, "egress: duplicate policy for provider operation")
		}

		policies[key] = policy
	}

	return &Limiter{
		limiter:  params.Limiter,
		policies: policies,
		clock:    clock,
		jitter:   jitter,
		sleeper:  sleeper,
	}, nil
}

// Wait blocks until the provider operation holds a token-bucket permit or a
// terminal condition is reached. Pre-canceled contexts perform no port call.
// A cancellation racing with an atomic successful reservation may consume
// that permit; permits are never refunded.
func (l *Limiter) Wait(ctx context.Context, provider, operation string) error {
	if err := ctx.Err(); err != nil {
		return errors.Join(entity.NewError(CodeContextCanceled, "egress: wait context ended before admission"), err)
	}

	policy, ok := l.policies[policyKey{provider: provider, operation: operation}]
	if !ok {
		return entity.NewError(CodeInvalidPolicy, "egress: no policy for provider operation")
	}

	key, err := domainservice.BuildRateLimitKey(provider, operation)
	if err != nil {
		return errors.Join(entity.NewError(CodeInvalidPolicy, "egress: provider operation is not a safe key"), err)
	}

	deadline := l.clock.Now().Add(policy.MaxWait)
	for {
		decision, err := l.limiter.Allow(ctx, key, policy.Budget, policy.Window)
		if err != nil {
			return errors.Join(entity.NewError(CodeStoreUnavailable, "egress: limiter store unavailable"), err)
		}

		if decision.Allowed {
			return nil
		}

		if decision.RetryAfter <= 0 {
			return entity.NewError(CodeMalformedDecision, "egress: limiter denied without a positive retry delay")
		}

		remaining := deadline.Sub(l.clock.Now())
		if remaining < decision.RetryAfter {
			return entity.NewError(CodeWaitExceeded, "egress: max wait exceeded before admission")
		}

		jitterUpper := min(decision.RetryAfter, remaining-decision.RetryAfter)

		wait := decision.RetryAfter + l.jitter.Jitter(jitterUpper)
		if err := l.sleeper.Sleep(ctx, wait); err != nil {
			return errors.Join(entity.NewError(CodeContextCanceled, "egress: wait context ended during admission"), err)
		}
	}
}
