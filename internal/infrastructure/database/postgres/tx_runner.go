package postgres

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/rls"
)

// WithinTx runs fn inside a database transaction on db with context cancellation check.
func WithinTx(ctx context.Context, db *gorm.DB, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("postgres: db is required")
	}

	if fn == nil {
		return fmt.Errorf("postgres: transaction callback is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("postgres: context canceled: %w", err)
	}

	return db.WithContext(ctx).Transaction(fn)
}

// WithinTenantTx runs fn inside a transaction scoped to tenant via RLS (SET LOCAL).
// It applies tenant isolation before invoking fn.
func WithinTenantTx(ctx context.Context, db *gorm.DB, tenant valueobject.TenantID, fn func(tx *gorm.DB) error) error {
	if db == nil {
		return fmt.Errorf("postgres: db is required")
	}

	if fn == nil {
		return fmt.Errorf("postgres: transaction callback is required")
	}

	if tenant.String() == "" {
		return fmt.Errorf("postgres: tenant is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("postgres: context canceled: %w", err)
	}

	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := rls.ApplyTenant(ctx, tx, tenant); err != nil {
			return err
		}

		return fn(tx)
	})
}
