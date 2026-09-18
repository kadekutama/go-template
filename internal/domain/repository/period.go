package repository

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PeriodRepository persists accounting periods. Open-period lookups feed
// posting-time validation and are strongly consistent.
type PeriodRepository interface {
	// Create stores a new period and returns it with the database-assigned ID.
	// Strong write; fails on duplicate ID.
	Create(ctx context.Context, period entity.PeriodData) (entity.PeriodData, error)
	// FindByID returns one period by tenant + ID. Strong read.
	FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.PeriodID) (entity.PeriodData, error)
	// FindOpen returns the currently open period of a ledger, if any.
	// Strong read.
	FindOpen(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID) (entity.PeriodData, error)
	// FindByDate returns the period containing t for a ledger. Strong read.
	FindByDate(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, t time.Time) (entity.PeriodData, error)
}
