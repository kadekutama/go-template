package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Subject is the command actor: a user, service account, or system job,
// always scoped to its tenant.
type Subject struct {
	// ID identifies the actor.
	ID string
	// TenantID scopes the actor to its tenant.
	TenantID valueobject.TenantID
}

// Authorizer is the command-level authorization boundary. Handlers check it
// before execution; enforcement details (Casbin, roles, grants) live with the
// adapter (E09). Denial is an error, never a boolean the caller can ignore.
type Authorizer interface {
	// Authorize permits subject to perform action on resource, or returns a
	// descriptive error. Strong read of policy state.
	Authorize(ctx context.Context, subject Subject, action, resource string) error
}
