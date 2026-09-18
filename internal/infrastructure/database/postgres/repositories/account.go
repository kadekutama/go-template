package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// isDuplicate reports unique-violation errors across drivers.
func isDuplicate(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToUpper(err.Error())

	return strings.Contains(msg, "DUPLICATE") ||
		strings.Contains(msg, "UNIQUE") ||
		strings.Contains(msg, "23505")
}

// AccountRepositoryParams carries constructor dependencies.
type AccountRepositoryParams struct {
	DB *gorm.DB
}

// AccountRepository is the PostgreSQL AccountRepository adapter.
type AccountRepository struct {
	db *gorm.DB
}

// NewAccountRepository builds the adapter; DB must be non-nil.
func NewAccountRepository(params AccountRepositoryParams) (*AccountRepository, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: account repository needs DB")
	}

	return &AccountRepository{db: params.DB}, nil
}

// Create stores a new account record and returns it with the
// database-assigned ID; duplicate IDs conflict.
func (r *AccountRepository) Create(ctx context.Context, account entity.AccountData) (entity.AccountData, error) {
	if err := account.Validate(); err != nil {
		return entity.AccountData{}, err
	}

	if err := scopeTenant(ctx, r.db, account.TenantID); err != nil {
		return entity.AccountData{}, err
	}

	model := accountToModel(account)
	model.Metadata = marshalMetadata(account.Metadata)

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return entity.AccountData{}, ErrConflict
		}

		return entity.AccountData{}, fmt.Errorf("postgres: create account: %w", err)
	}

	return accountToEntity(model), nil
}

// FindByID returns one account by tenant + ID (strong read).
func (r *AccountRepository) FindByID(
	ctx context.Context,
	tenant valueobject.TenantID,
	id valueobject.AccountID,
) (entity.AccountData, error) {
	if tenant.String() == "" {
		return entity.AccountData{}, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return entity.AccountData{}, err
	}

	var model models.AccountModel

	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id.String(), tenant.String()).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.AccountData{}, ErrNotFound
		}

		return entity.AccountData{}, fmt.Errorf("postgres: find account: %w", err)
	}

	return accountToEntity(model), nil
}

// FindByTenant returns one opaque-cursor page of accounts.
func (r *AccountRepository) FindByTenant(
	ctx context.Context,
	tenant valueobject.TenantID,
	cursor string,
	limit int,
) ([]entity.AccountData, string, error) {
	if tenant.String() == "" {
		return nil, "", fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return nil, "", err
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var rows []models.AccountModel

	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenant.String()).Order("id")
	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}

	if err := query.Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", fmt.Errorf("postgres: list accounts: %w", err)
	}

	next := ""

	if len(rows) > limit {
		next = rows[limit-1].ID
		rows = rows[:limit]
	}

	out := make([]entity.AccountData, 0, len(rows))
	for _, row := range rows {
		out = append(out, accountToEntity(row))
	}

	return out, next, nil
}

// UpdateMetadata replaces name/purpose guarded by the expected version.
func (r *AccountRepository) UpdateMetadata(ctx context.Context, account entity.AccountData, expectedVersion int64) error {
	if err := account.Validate(); err != nil {
		return err
	}

	res := r.db.WithContext(ctx).Model(&models.AccountModel{}).
		Where("id = ? AND tenant_id = ? AND version = ?", account.ID.String(), account.TenantID.String(), expectedVersion).
		Updates(map[string]any{
			"name":          account.Name,
			"purpose":       account.Purpose,
			columnVersion:   expectedVersion + 1,
			columnUpdatedAt: gorm.Expr("now()"),
		})
	if res.Error != nil {
		return fmt.Errorf("postgres: update account metadata: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return ErrVersionMismatch
	}

	return nil
}

// UpdateStatus transitions lifecycle status guarded by the expected version.
func (r *AccountRepository) UpdateStatus(
	ctx context.Context,
	tenant valueobject.TenantID,
	id valueobject.AccountID,
	status valueobject.AccountStatus,
	expectedVersion int64,
) error {
	if tenant.String() == "" {
		return fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return err
	}

	res := r.db.WithContext(ctx).Model(&models.AccountModel{}).
		Where("id = ? AND tenant_id = ? AND version = ?", id.String(), tenant.String(), expectedVersion).
		Updates(map[string]any{
			"status":        string(status),
			columnVersion:   expectedVersion + 1,
			columnUpdatedAt: gorm.Expr("now()"),
		})
	if res.Error != nil {
		return fmt.Errorf("postgres: update account status: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return ErrVersionMismatch
	}

	return nil
}

func accountToModel(account entity.AccountData) models.AccountModel {
	parent := ""
	if account.ParentID != nil {
		parent = account.ParentID.String()
	}

	var parentPtr *string
	if parent != "" {
		parentPtr = &parent
	}

	return models.AccountModel{
		ID:        account.ID.String(),
		TenantID:  account.TenantID.String(),
		LedgerID:  account.LedgerID.String(),
		ParentID:  parentPtr,
		Number:    account.Number,
		Name:      account.Name,
		Class:     string(account.Class),
		AssetCode: string(account.AssetCode),
		Status:    string(account.Status),
		Purpose:   account.Purpose,
		Metadata:  marshalMetadata(account.Metadata),
		Version:   account.Version,
		CreatedAt: account.CreatedAt,
		UpdatedAt: account.UpdatedAt,
	}
}

func accountToEntity(model models.AccountModel) entity.AccountData {
	account := entity.AccountData{
		ID:        valueobject.AccountID(model.ID),
		TenantID:  valueobject.TenantID(model.TenantID),
		LedgerID:  valueobject.LedgerID(model.LedgerID),
		Number:    model.Number,
		Name:      model.Name,
		Class:     valueobject.AccountClass(model.Class),
		AssetCode: valueobject.AssetCode(model.AssetCode),
		Status:    valueobject.AccountStatus(model.Status),
		Purpose:   model.Purpose,
		Version:   model.Version,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}

	if model.ParentID != nil && *model.ParentID != "" {
		parent := valueobject.AccountID(*model.ParentID)
		account.ParentID = &parent
	}

	return account
}

// Compile-time port assertion.
var _ repository.AccountRepository = (*AccountRepository)(nil)
