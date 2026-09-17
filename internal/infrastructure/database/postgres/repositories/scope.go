package repositories

import (
	"context"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/rls"
)

// scopeTenant verifies agreement between context and argument tenants via
// rls.ResolveTenant (failing closed on mismatch or blank) and applies the
// tenant setting for RLS policies (migration 000004).
func scopeTenant(ctx context.Context, db *gorm.DB, tenant valueobject.TenantID) error {
	resolved, err := rls.ResolveTenant(ctx, tenant)
	if err != nil {
		return err
	}

	return rls.ApplyTenant(ctx, db, resolved)
}
