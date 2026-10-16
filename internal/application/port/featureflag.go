package port

import (
	"context"
)

// FlagContext carries evaluation attributes for one flag read. Only
// non-sensitive routing attributes belong here: user and tenant identifiers
// plus custom string attributes (payment method, currency, channel). Secrets,
// tokens, and PII MUST NOT be placed in Attributes; the adapter never logs
// or persists evaluation context.
type FlagContext struct {
	// UserID scopes evaluation to one user. Empty means unauthenticated.
	UserID string
	// TenantID scopes evaluation to one tenant. Empty means global.
	TenantID string
	// Attributes carries custom string attributes. Nil is valid.
	Attributes map[string]string
}

// FlagDetails is the evaluated flag value plus its provenance. Fallback is
// true when the value came from last-known or configured defaults instead of
// Unleash; Stale is true when last-known is past the refresh bound. Callers
// decide fail-open versus fail-closed from these markers.
type FlagDetails struct {
	// Value is the evaluated flag value.
	Value bool
	// Fallback is true when served from last-known or defaults, not Unleash.
	Fallback bool
	// Stale is true when last-known is past the refresh bound.
	Stale bool
}

// FlagClient is the runtime flag boundary (OpenFeature + Unleash in E10-T01,
// ADR-014 edge excluded). Reads are cacheable hints: kill switches and
// method/currency gates may change display or routing behavior but MUST NOT
// rewrite postings, balances, or idempotency records.
//
// Timeout and retry ownership: callers set deadlines via ctx; adapters honor
// them, bound retries with an explicit policy, and never retry cancellation.
// Consistency: evaluations are eventually consistent within the refresh bound;
// fallback values carry explicit markers and never claim to be fresh.
// Idempotency: flag reads are idempotent cacheable reads with no
// money-movement semantics. Sensitive data: evaluation context, keys, and
// provider credentials MUST NOT appear in logs, errors, egress keys, or
// cached entries.
type FlagClient interface {
	// Boolean evaluates one boolean flag, returning the configured default
	// when the provider is unreachable. A non-nil error carries the typed
	// failure alongside the fallback value.
	Boolean(ctx context.Context, key string, def bool, evalCtx FlagContext) (bool, error)
	// BooleanWithDetails evaluates one boolean flag with provenance markers.
	BooleanWithDetails(ctx context.Context, key string, def bool, evalCtx FlagContext) (FlagDetails, error)
	// Close releases provider resources. It is safe to call more than once.
	Close(ctx context.Context) error
}
