// Package repositories implements the domain persistence ports on GORM.
// Every method is tenant-scoped; a missing tenant fails closed.
package repositories

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// ErrNotFound marks a missing row. Callers translate it at the edge.
var ErrNotFound = errors.New("postgres: not found")

// ErrConflict marks a duplicate-ID write.
var ErrConflict = errors.New("postgres: duplicate id")

// ErrVersionMismatch marks a failed optimistic-concurrency guard.
var ErrVersionMismatch = errors.New("postgres: version mismatch")

// LedgerRepositoryParams carries constructor dependencies.
type LedgerRepositoryParams struct {
	DB *gorm.DB
}

// LedgerRepository is the PostgreSQL LedgerRepository adapter.
type LedgerRepository struct {
	db *gorm.DB
}

// NewLedgerRepository builds the adapter; DB must be non-nil.
func NewLedgerRepository(params LedgerRepositoryParams) (*LedgerRepository, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: ledger repository needs DB")
	}

	return &LedgerRepository{db: params.DB}, nil
}

// Create stores a new ledger; duplicate IDs conflict.
func (r *LedgerRepository) Create(ctx context.Context, ledger entity.Ledger) error {
	if _, err := entity.NewLedger(ledger.ID, ledger.TenantID, ledger.Name, ledger.BaseAsset, ledger.ChartVersion); err != nil {
		return err
	}

	model := models.LedgerToModel(ledger)

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return ErrConflict
		}

		return fmt.Errorf("postgres: create ledger: %w", err)
	}

	return nil
}

// FindByID returns one ledger by tenant + ID (strong read).
func (r *LedgerRepository) FindByID(ctx context.Context, tenant valueobject.TenantID, id valueobject.LedgerID) (entity.Ledger, error) {
	if tenant.String() == "" {
		return entity.Ledger{}, fmt.Errorf("postgres: tenant is required")
	}

	var model models.LedgerModel

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return entity.Ledger{}, err
	}

	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id.String(), tenant.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.Ledger{}, ErrNotFound
		}

		return entity.Ledger{}, fmt.Errorf("postgres: find ledger: %w", err)
	}

	ledger, err := models.LedgerToEntity(model)
	if err != nil {
		return entity.Ledger{}, fmt.Errorf("postgres: invalid ledger row %s: %w", model.ID, err)
	}

	return ledger, nil
}

// ListByTenant returns all ledgers of a tenant (strong read, small result).
func (r *LedgerRepository) ListByTenant(ctx context.Context, tenant valueobject.TenantID) ([]entity.Ledger, error) {
	if tenant.String() == "" {
		return nil, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return nil, err
	}

	var rows []models.LedgerModel

	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenant.String()).Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("postgres: list ledgers: %w", err)
	}

	out := make([]entity.Ledger, 0, len(rows))
	for _, row := range rows {
		ledger, err := models.LedgerToEntity(row)
		if err != nil {
			return nil, fmt.Errorf("postgres: invalid ledger row %s: %w", row.ID, err)
		}

		out = append(out, ledger)
	}

	return out, nil
}

// Compile-time port assertions.
var (
	_ repository.LedgerRepository = (*LedgerRepository)(nil)
)
