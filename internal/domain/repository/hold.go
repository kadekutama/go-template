package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// HoldRepository persists durable holds. All reads used for spend decisions
// are strongly consistent.
type HoldRepository interface {
	// Create stores a new ACTIVE hold and returns it with the database-assigned ID.
	// Strong write; fails on duplicate ID.
	Create(ctx context.Context, hold entity.HoldData) (entity.HoldData, error)
	// FindByID returns one hold by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.HoldID) (entity.HoldData, error)
	// FindActiveByAccount returns ACTIVE holds blocking an account. Strong read.
	FindActiveByAccount(ctx context.Context, tenant valueobject.TenantID, account valueobject.AccountID) ([]entity.HoldData, error)
	// Update stores a state transition guarded by the expected version.
	// Fails on version mismatch. Strong write.
	Update(ctx context.Context, hold entity.HoldData, expectedVersion int64) error
}
