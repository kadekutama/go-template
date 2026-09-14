package repository

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// JournalRepository persists posting groups. Journals are operational records;
// reads are strongly consistent.
type JournalRepository interface {
	// Create stores a new journal. Strong write; fails on duplicate ID.
	Create(ctx context.Context, journal entity.Journal) error
	// FindByID returns one journal by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.JournalID) (entity.Journal, error)
	// FindByPeriod returns journals filed in a period. Strong read.
	FindByPeriod(ctx context.Context, tenant valueobject.TenantID, period valueobject.PeriodID) ([]entity.Journal, error)
}
