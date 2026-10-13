package admin

import (
	"context"
	"time"
)

// Denial reasons recorded for rejected scheduler-control attempts. Reasons
// name the policy outcome only; events never carry credentials, tokens, or
// request secrets.
const (
	// ReasonAuthRequired marks requests without a usable session.
	ReasonAuthRequired = "auth_required"
	// ReasonForbidden marks authenticated requests denied by the read-only
	// policy or missing scope.
	ReasonForbidden = "forbidden"
	// ReasonUnknownRoute marks denied requests to unknown /api/* paths.
	ReasonUnknownRoute = "unknown_route"
	// ReasonOriginRejected marks requests failing exact Host/Origin checks.
	ReasonOriginRejected = "origin_rejected"
	// ReasonRateLimited marks requests denied by the UI rate limiter.
	ReasonRateLimited = "rate_limited"
	// ReasonLimiterUnavailable marks fail-closed limiter store errors.
	ReasonLimiterUnavailable = "limiter_unavailable"
)

// DeniedControlEvent records one rejected scheduler-control attempt for the
// security audit chain. Actor is the validated opaque subject ID or
// "anonymous"; no PII, usernames, emails, or secrets are recorded.
type DeniedControlEvent struct {
	OccurredAt time.Time
	Actor      string
	Method     string
	Path       string
	Reason     string
}

// AuditLogger receives denied-control events. The concrete audit adapter
// lives behind the composition root; audit failure fails closed.
type AuditLogger interface {
	LogDeniedControl(ctx context.Context, event DeniedControlEvent) error
}
