// Package rls enforces tenant-scoped PostgreSQL access (E07-T02): tenant
// context helpers plus SET LOCAL scoping for the RLS policies in 000004.
package rls

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// tenantKey is the unexported context key for the current tenant.
type tenantKey struct{}

// SettingName is the PostgreSQL setting RLS policies key on.
const SettingName = "app.current_tenant"

// WithTenant stores the tenant in context. Blank IDs are rejected.
func WithTenant(ctx context.Context, tenant valueobject.TenantID) (context.Context, error) {
	if tenant.String() == "" {
		return nil, fmt.Errorf("rls: tenant is required")
	}

	return context.WithValue(ctx, tenantKey{}, tenant), nil
}

// TenantFromContext resolves the tenant or fails closed.
func TenantFromContext(ctx context.Context) (valueobject.TenantID, error) {
	tenant, ok := ctx.Value(tenantKey{}).(valueobject.TenantID)
	if !ok || tenant.String() == "" {
		return "", fmt.Errorf("rls: tenant missing from context")
	}

	return tenant, nil
}

// ResolveTenant agrees a context tenant with an explicit argument tenant.
// Both present and different is a confused-deputy error; both absent fails.
func ResolveTenant(ctx context.Context, arg valueobject.TenantID) (valueobject.TenantID, error) {
	fromCtx, ctxErr := TenantFromContext(ctx)

	switch {
	case ctxErr == nil && arg.String() == "":
		return fromCtx, nil
	case ctxErr != nil && arg.String() != "":
		return arg, nil
	case ctxErr == nil && fromCtx == arg:
		return fromCtx, nil
	case ctxErr == nil:
		return "", fmt.Errorf("rls: context tenant and argument tenant disagree")
	default:
		return "", fmt.Errorf("rls: tenant is required")
	}
}

// ApplyTenant scopes the current transaction to the tenant via SET LOCAL.
// It must run inside the repository transaction before any user-table access.
func ApplyTenant(ctx context.Context, db *gorm.DB, tenant valueobject.TenantID) error {
	if tenant.String() == "" {
		return fmt.Errorf("rls: tenant is required")
	}

	if db == nil {
		return fmt.Errorf("rls: DB is required")
	}

	if err := db.WithContext(ctx).Exec("SELECT set_config(?, ?, true)", SettingName, tenant.String()).Error; err != nil {
		return fmt.Errorf("rls: apply tenant: %w", err)
	}

	return nil
}

// ApplyContextTenant resolves then applies the context tenant.
func ApplyContextTenant(ctx context.Context, db *gorm.DB) error {
	tenant, err := TenantFromContext(ctx)
	if err != nil {
		return err
	}

	return ApplyTenant(ctx, db, tenant)
}
