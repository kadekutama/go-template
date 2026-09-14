// Package repository holds the domain-owned persistence ports every
// infrastructure adapter implements. All methods are tenant-scoped, take
// context first, and return errors; consistency expectations are documented
// per method.
package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// LedgerRepository persists ledger boundaries. Reads are strongly consistent;
// ledgers are few and always read from the primary.
type LedgerRepository interface {
	// Create stores a new ledger. Strong write; fails on duplicate ID.
	Create(ctx context.Context, ledger entity.Ledger) error
	// FindByID returns one ledger by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.LedgerID) (entity.Ledger, error)
	// ListByTenant returns all ledgers of a tenant. Strong read; small result.
	ListByTenant(ctx context.Context, tenant valueobject.TenantID) ([]entity.Ledger, error)
}
