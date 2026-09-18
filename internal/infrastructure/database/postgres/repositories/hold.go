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

// HoldRepositoryParams carries constructor dependencies.
type HoldRepositoryParams struct {
	DB *gorm.DB
}

// HoldRepository is the PostgreSQL HoldRepository adapter.
type HoldRepository struct {
	db *gorm.DB
}

// NewHoldRepository builds the adapter; DB must be non-nil.
func NewHoldRepository(params HoldRepositoryParams) (*HoldRepository, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: hold repository needs DB")
	}

	return &HoldRepository{db: params.DB}, nil
}

// Create stores a new ACTIVE hold and returns it with the
// database-assigned ID; duplicate IDs conflict.
func (r *HoldRepository) Create(ctx context.Context, hold entity.HoldData) (entity.HoldData, error) {
	if err := hold.Validate(); err != nil {
		return entity.HoldData{}, err
	}

	if err := scopeTenant(ctx, r.db, hold.TenantID); err != nil {
		return entity.HoldData{}, err
	}

	model := holdToModel(hold)

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return entity.HoldData{}, ErrConflict
		}

		return entity.HoldData{}, fmt.Errorf("postgres: create hold: %w", err)
	}

	return holdToEntity(model), nil
}

// FindByID returns one hold by tenant + ID (strong read).
func (r *HoldRepository) FindByID(
	ctx context.Context,
	tenant valueobject.TenantID,
	id valueobject.HoldID,
) (entity.HoldData, error) {
	if tenant.String() == "" {
		return entity.HoldData{}, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return entity.HoldData{}, err
	}

	var model models.HoldModel

	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id.String(), tenant.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.HoldData{}, ErrNotFound
		}

		return entity.HoldData{}, fmt.Errorf("postgres: find hold: %w", err)
	}

	return holdToEntity(model), nil
}

// FindActiveByAccount returns ACTIVE holds blocking an account (strong read).
func (r *HoldRepository) FindActiveByAccount(
	ctx context.Context,
	tenant valueobject.TenantID,
	account valueobject.AccountID,
) ([]entity.HoldData, error) {
	if tenant.String() == "" {
		return nil, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return nil, err
	}

	var rows []models.HoldModel

	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND account_id = ? AND state = ?", tenant.String(), account.String(), entity.HoldActive).
		Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("postgres: list active holds: %w", err)
	}

	out := make([]entity.HoldData, 0, len(rows))
	for _, row := range rows {
		out = append(out, holdToEntity(row))
	}

	return out, nil
}

// Update stores a state transition guarded by the expected version.
func (r *HoldRepository) Update(ctx context.Context, hold entity.HoldData, expectedVersion int64) error {
	if err := hold.Validate(); err != nil {
		return err
	}

	res := r.db.WithContext(ctx).Model(&models.HoldModel{}).
		Where("id = ? AND tenant_id = ? AND version = ?", hold.ID.String(), hold.TenantID.String(), expectedVersion).
		Updates(map[string]any{
			"state":         hold.State,
			columnVersion:   expectedVersion + 1,
			columnUpdatedAt: gorm.Expr("now()"),
		})
	if res.Error != nil {
		return fmt.Errorf("postgres: update hold: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return ErrVersionMismatch
	}

	return nil
}

func holdToModel(hold entity.HoldData) models.HoldModel {
	return models.HoldModel{
		ID:          hold.ID.String(),
		TenantID:    hold.TenantID.String(),
		LedgerID:    hold.LedgerID.String(),
		AccountID:   hold.AccountID.String(),
		AssetCode:   string(hold.AssetCode),
		AmountMinor: hold.AmountMinor,
		Kind:        hold.Kind,
		State:       hold.State,
		ExpiresAt:   hold.ExpiresAt,
		Version:     hold.Version,
		CreatedAt:   hold.CreatedAt,
		UpdatedAt:   hold.UpdatedAt,
	}
}

func holdToEntity(model models.HoldModel) entity.HoldData {
	return entity.HoldData{
		ID:          valueobject.HoldID(model.ID),
		TenantID:    valueobject.TenantID(model.TenantID),
		LedgerID:    valueobject.LedgerID(model.LedgerID),
		AccountID:   valueobject.AccountID(model.AccountID),
		AssetCode:   valueobject.AssetCode(model.AssetCode),
		AmountMinor: model.AmountMinor,
		Kind:        model.Kind,
		State:       model.State,
		ExpiresAt:   model.ExpiresAt,
		Version:     model.Version,
		CreatedAt:   model.CreatedAt,
		UpdatedAt:   model.UpdatedAt,
	}
}

// Compile-time port assertion.
var _ repository.HoldRepository = (*HoldRepository)(nil)
