package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// AccountRepository persists account classification records. Point lookups are
// strongly consistent; listings are point-in-time pages with opaque cursors.
type AccountRepository interface {
	// Create stores a new account record and returns it with the database-assigned ID.
	// Strong write; fails on duplicate ID.
	Create(ctx context.Context, account entity.AccountData) (entity.AccountData, error)
	// FindByID returns one account by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error)
	// FindByTenant returns one page of accounts. Point-in-time page; the
	// returned cursor addresses the next page and must be treated as opaque.
	FindByTenant(ctx context.Context, tenant valueobject.TenantID, cursor string, limit int) ([]entity.AccountData, string, error)
	// UpdateMetadata replaces name/purpose/metadata guarded by the expected
	// version. Fails on version mismatch. Strong write.
	UpdateMetadata(ctx context.Context, account entity.AccountData, expectedVersion int64) error
	// UpdateStatus transitions lifecycle status guarded by the expected
	// version. Fails on version mismatch. Strong write.
	UpdateStatus(ctx context.Context, tenant valueobject.TenantID, id valueobject.AccountID, status valueobject.AccountStatus, expectedVersion int64) error
}
