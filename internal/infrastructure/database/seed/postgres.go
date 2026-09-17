package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
	repos "github.com/kadekutama/go-template/internal/infrastructure/database/postgres/repositories"
)

// PostgresStores provides a GORM-backed implementation of Stores.
func PostgresStores(db *gorm.DB) Stores {
	return Stores{
		Ledgers:  &pgLedgerStore{db: db},
		Accounts: &pgAccountStore{db: db},
		Postings: &pgPostingStore{db: db},
	}
}

type pgLedgerStore struct {
	db *gorm.DB
}

func (s *pgLedgerStore) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.LedgerModel{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("postgres seed: check ledger %s: %w", id, err)
	}
	return count > 0, nil
}

func (s *pgLedgerStore) Create(ctx context.Context, ledger LedgerSeed) error {
	repo, err := repos.NewLedgerRepository(repos.LedgerRepositoryParams{DB: s.db})
	if err != nil {
		return err
	}

	entityLedger, err := entity.NewLedger(
		valueobject.LedgerID(ledger.ID),
		valueobject.TenantID(ledger.TenantID),
		ledger.Name,
		valueobject.AssetCode(ledger.BaseAsset),
		ledger.ChartVersion,
	)
	if err != nil {
		return fmt.Errorf("postgres seed: build ledger: %w", err)
	}

	if err := repo.Create(ctx, entityLedger); err != nil {
		if errors.Is(err, repos.ErrConflict) {
			return nil
		}
		return fmt.Errorf("postgres seed: create ledger: %w", err)
	}

	return nil
}

type pgAccountStore struct {
	db *gorm.DB
}

func (s *pgAccountStore) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.AccountModel{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("postgres seed: check account %s: %w", id, err)
	}
	return count > 0, nil
}

func (s *pgAccountStore) Create(ctx context.Context, account AccountSeed) error {
	repo, err := repos.NewAccountRepository(repos.AccountRepositoryParams{DB: s.db})
	if err != nil {
		return err
	}

	accountData := entity.AccountData{
		ID:        valueobject.AccountID(account.ID),
		TenantID:  valueobject.TenantID(account.TenantID),
		LedgerID:  valueobject.LedgerID(account.LedgerID),
		Number:    account.Number,
		Name:      account.Name,
		Class:     valueobject.AccountClass(account.Class),
		AssetCode: valueobject.AssetCode(account.AssetCode),
		Status:    valueobject.AccountStatus(account.Status),
		Metadata:  map[string]string{},
		Version:   account.Version,
	}

	if err := repo.Create(ctx, accountData); err != nil {
		if errors.Is(err, repos.ErrConflict) {
			return nil
		}
		return fmt.Errorf("postgres seed: create account %s: %w", account.ID, err)
	}

	return nil
}

type pgPostingStore struct {
	db *gorm.DB
}

func (s *pgPostingStore) Exists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&models.PostingModel{}).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("postgres seed: check posting %s: %w", id, err)
	}
	return count > 0, nil
}

func (s *pgPostingStore) Create(ctx context.Context, posting PostingSeed) error {
	repo, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: s.db})
	if err != nil {
		return err
	}

	now := time.Now()
	postingData := entity.PostingData{
		ID:          valueobject.PostingID(posting.ID),
		TenantID:    valueobject.TenantID(posting.TenantID),
		LedgerID:    valueobject.LedgerID(posting.LedgerID),
		Operation:   posting.Operation,
		Description: posting.Description,
		Entries: []entity.Entry{
			{
				ID:          valueobject.EntryID(posting.ID + "-dr"),
				PostingID:   valueobject.PostingID(posting.ID),
				AccountID:   valueobject.AccountID(posting.Debit.ID),
				Side:        valueobject.DirectionDebit,
				AmountMinor: posting.AmountMinor,
				AssetCode:   valueobject.AssetCode(posting.AssetCode),
			},
			{
				ID:          valueobject.EntryID(posting.ID + "-cr"),
				PostingID:   valueobject.PostingID(posting.ID),
				AccountID:   valueobject.AccountID(posting.Credit.ID),
				Side:        valueobject.DirectionCredit,
				AmountMinor: posting.AmountMinor,
				AssetCode:   valueobject.AssetCode(posting.AssetCode),
			},
		},
		EffectiveAt: now,
		RecordedAt:  now,
		Metadata:    map[string]string{},
	}

	if err := repo.Commit(ctx, postingData); err != nil {
		if errors.Is(err, repos.ErrConflict) {
			return nil
		}
		return fmt.Errorf("postgres seed: commit posting %s: %w", posting.ID, err)
	}

	return nil
}

// ApplyPostgres applies the deterministic seed plan against a real PostgreSQL database.
// It ensures referenced assets and the tenant row exist before applying chart accounts and postings.
func ApplyPostgres(ctx context.Context, db *gorm.DB, plan Plan) error {
	if db == nil {
		return fmt.Errorf("postgres seed: DB is required")
	}

	// 1. Ensure assets exist
	for _, assetCode := range []string{AssetUSD, AssetEUR, AssetIDR} {
		asset := models.AssetModel{
			Code:      assetCode,
			Precision: 2,
			Status:    "ACTIVE",
		}
		if err := db.WithContext(ctx).Where("code = ?", assetCode).FirstOrCreate(&asset).Error; err != nil {
			return fmt.Errorf("postgres seed: ensure asset %s: %w", assetCode, err)
		}
	}

	// 2. Ensure tenant exists
	tenant := models.TenantModel{
		ID:       plan.Tenant.TenantID,
		Name:     plan.Tenant.Name,
		Region:   plan.Tenant.Region,
		Settings: "{}",
		Status:   "ACTIVE",
		Version:  1,
	}
	if err := db.WithContext(ctx).Where("id = ?", plan.Tenant.TenantID).FirstOrCreate(&tenant).Error; err != nil {
		return fmt.Errorf("postgres seed: ensure tenant %s: %w", plan.Tenant.TenantID, err)
	}

	// 3. Apply ledgers, accounts, postings
	stores := PostgresStores(db)
	return Apply(ctx, plan, stores)
}
