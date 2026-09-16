package command

import (
	"context"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// RequireAuthz enforces command-level authorization before execution.
// Empty subject identities fail closed without consulting the adapter;
// everything else delegates to the Authorizer port.
func RequireAuthz(ctx context.Context, authz port.Authorizer, subject port.Subject, action, resource string) error {
	if strings.TrimSpace(subject.ID) == "" {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return authz.Authorize(ctx, subject, action, resource)
}
