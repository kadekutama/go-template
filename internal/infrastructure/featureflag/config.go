// Package featureflag provides the OpenFeature + Unleash flag adapter behind
// the application FlagClient port (E10-T01, SPEC §7.6/§7.10). Every outbound
// Unleash HTTP request acquires an E09.1-T03 egress permit before the attempt
// and runs inside the E01-T08 breaker; limiter-store errors fail closed to the
// Valkey/in-memory fallback without an unmetered request. Flags are
// non-authoritative hints and never rewrite postings or balances.
package featureflag

import (
	"context"
	"net/http"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/infrastructure/egress"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	kernelresilience "github.com/kadekutama/go-template/internal/shared/kernel/resilience"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Typed error codes for flag failures. Egress denials keep their EGRESS_*
// codes so callers can distinguish quota exhaustion from provider outages;
// all other provider failures use the FEATUREFLAG_* codes below with the
// fallback value served alongside the error.
const (
	// CodeInvalidConfig marks missing or malformed wiring and policy.
	CodeInvalidConfig = "FEATUREFLAG_INVALID_CONFIG"
	// CodeInvalidFlagKey marks an empty flag key.
	CodeInvalidFlagKey = "FEATUREFLAG_INVALID_FLAG_KEY"
	// CodeProviderUnavailable marks Unleash unreachable or not ready.
	CodeProviderUnavailable = "FEATUREFLAG_PROVIDER_UNAVAILABLE"
	// CodeBreakerOpen marks fast-failed calls while the breaker is open.
	CodeBreakerOpen = "FEATUREFLAG_BREAKER_OPEN"
)

// Provider and operation names for the egress limiter. Keys become
// ratelimit:unleash:flags through the E05 key builder; they carry no tenant
// PII.
const (
	// EgressProvider is the Unleash provider key segment.
	EgressProvider = "unleash"
	// EgressOperation is the flag-fetch operation key segment.
	EgressOperation = "flags"
)

// EgressWaiter admits one provider operation. Production wires the E09.1-T03
// egress limiter; tests supply a scripted fake to prove permit-per-attempt
// and fail-closed behavior without Docker.
type EgressWaiter interface {
	// Wait blocks until the provider operation holds a permit or a terminal
	// condition is reached. Limiter errors are terminal and must never lead
	// to an unmetered provider call.
	Wait(ctx context.Context, provider, operation string) error
}

// Evaluator evaluates one boolean flag against the provider. Production uses
// OpenFeature + Unleash; tests supply a scripted fake. A nil error means the
// value came from the provider; a non-nil error means the caller must serve
// fallback and surface the error alongside it.
type Evaluator interface {
	// Evaluate returns the provider value or a typed failure.
	Evaluate(ctx context.Context, key string, def bool, evalCtx appport.FlagContext) (bool, error)
	// Health reports provider readiness without credentials in logs.
	Health(ctx context.Context) error
	// Close releases provider resources. It is safe to call more than once.
	Close(ctx context.Context) error
}

// Params wires a Client (Parameter Object pattern). Data fields carry
// validate tags and are checked through the shared validator; dependency
// seams (interfaces, maps of defaults excluded) get explicit nil checks so
// callers can errors.Is against a named error.
type Params struct {
	// UnleashURL is the Unleash server base URL (e.g. http://unleash:4242/api).
	UnleashURL string `validate:"required,url"`
	// AppName is the Unleash client application name.
	AppName string `validate:"required,min=1,max=64"`
	// InstanceID identifies this client instance. Empty uses hostname.
	InstanceID string `validate:"omitempty,max=64"`
	// Environment scopes flag evaluation (e.g. development). Empty is global.
	Environment string `validate:"omitempty,max=32"`
	// RefreshInterval bounds staleness (10s per epic). Explicit, no default.
	RefreshInterval time.Duration `validate:"required,gt=0"`
	// RetryPolicy bounds provider-attempt retries. Explicit, no zero defaults.
	RetryPolicy kernelresilience.RetryPolicy
	// Egress admits every outbound attempt. Required; failures fail closed.
	Egress EgressWaiter `validate:"-"`
	// Breaker guards the provider dependency. Required.
	Breaker kernelresilience.Breaker `validate:"-"`
	// HTTP performs outbound Unleash requests. Required.
	HTTP httpclient.Doer `validate:"-"`
	// Clock supplies time for staleness. Required. It is the application
	// clock port (implemented by the shared kernel clock); no local clock
	// type exists in this package.
	Clock appport.Clock `validate:"-"`
	// Cache optionally persists last-known values (Valkey-backed in E08).
	// Nil means in-memory only; never Redis as a required dependency.
	Cache appport.Cache `validate:"-"`
	// CacheTTL bounds L2 retention for last-known values. Required positive:
	// every environment configures it explicitly; there is no default.
	CacheTTL time.Duration `validate:"required,gt=0"`
	// InitTimeout bounds the provider's initial toggle load wait. Explicit,
	// no default: a healthy server answers in milliseconds, while an
	// unreachable server must not stall construction past this bound.
	InitTimeout time.Duration `validate:"required,gt=0"`
	// ClientToken authenticates SDK polling via the Authorization header.
	// Required against real servers; empty only for auth-disabled instances.
	// Optional so local auth-disabled development needs no credential.
	ClientToken string
	// ProbeMaxBodyBytes bounds drained probe response bodies. Required
	// positive: every environment configures it explicitly; there is no
	// default.
	ProbeMaxBodyBytes int `validate:"required,gt=0"`
	// Transport is the base round tripper wrapped by the egress-gated
	// transport. Required: nil is rejected, never silently replaced with
	// http.DefaultTransport; production passes it explicitly.
	Transport http.RoundTripper `validate:"-"`
	// Logger records cache-write, drain, and breaker-bookkeeping failures.
	// Nil-tolerant: a nil logger disables failure logging. Flag keys, values,
	// and credentials never appear in log lines. Excluded from validator
	// traversal like all seams.
	Logger log.Logger `validate:"-"`
	// Defaults supplies configured fallback values per flag. Nil is valid.
	Defaults map[string]bool
	// Fallback supplies a pre-built fallback store, used by tests to seed
	// last-known values through production Observe. Nil builds a fresh store
	// from Clock/RefreshInterval/Defaults/Cache/CacheTTL (precedent: the
	// Evaluator test seam above).
	Fallback *Fallback `validate:"-"`
	// Evaluator overrides OpenFeature evaluation in tests. Nil uses Unleash.
	Evaluator Evaluator `validate:"-"`
}

// Validate checks Params deterministically. It reports data-field violations
// through the shared validator and seam violations as typed config errors.
func (p Params) Validate() error {
	if err := validate.Struct("featureflag", "params", p); err != nil {
		return err
	}

	if p.Egress == nil {
		return entity.NewError(CodeInvalidConfig, "featureflag: egress limiter is required")
	}

	if p.Breaker == nil {
		return entity.NewError(CodeInvalidConfig, "featureflag: breaker is required")
	}

	if p.HTTP == nil {
		return entity.NewError(CodeInvalidConfig, "featureflag: http client is required")
	}

	if p.Clock == nil {
		return entity.NewError(CodeInvalidConfig, "featureflag: clock is required")
	}

	if p.Transport == nil {
		return entity.NewError(CodeInvalidConfig, "featureflag: transport is required")
	}

	if err := p.RetryPolicy.Validate(); err != nil {
		return err
	}

	return nil
}

// LoadEgressPolicies parses one task-owned egress policy document (for
// example config/egress/unleash.yaml). A missing or invalid policy is a typed
// error, never an implicit unlimited quota.
func LoadEgressPolicies(data []byte) ([]egress.Policy, error) {
	policies, err := egress.ParsePolicies(data)
	if err != nil {
		return nil, err
	}

	for _, policy := range policies {
		if policy.Provider != EgressProvider || policy.Operation != EgressOperation {
			return nil, entity.NewError(CodeInvalidConfig, "featureflag: egress policy must be unleash/flags")
		}
	}

	return policies, nil
}

// newConfigError builds a typed configuration failure.
func newConfigError(message string) error {
	return entity.NewError(CodeInvalidConfig, message)
}
