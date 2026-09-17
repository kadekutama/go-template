package repositories

import (
	"context"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// EntryStoreParams carries constructor dependencies.
type EntryStoreParams struct {
	DB *gorm.DB
}

// EntryStore is the PostgreSQL EntryReader adapter. There is intentionally no
// entry write port: entries persist only through PostingRepository.Commit.
type EntryStore struct {
	db *gorm.DB
}

// NewEntryStore builds the adapter; DB must be non-nil.
func NewEntryStore(params EntryStoreParams) (*EntryStore, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: entry store needs DB")
	}

	return &EntryStore{db: params.DB}, nil
}

// FindByPosting returns all entries of one posting (strong read).
func (s *EntryStore) FindByPosting(
	ctx context.Context,
	tenant valueobject.TenantID,
	posting valueobject.PostingID,
) ([]entity.Entry, error) {
	return findEntriesByPosting(ctx, s.db, tenant, posting)
}

// FindByAccount returns one opaque-cursor page of entries for an account.
// The cursor is the last seen account_seq (decimal string); pages follow
// entry insertion order per account.
func (s *EntryStore) FindByAccount(
	ctx context.Context,
	tenant valueobject.TenantID,
	account valueobject.AccountID,
	cursor string,
	limit int,
) ([]entity.Entry, string, error) {
	if tenant.String() == "" {
		return nil, "", fmt.Errorf("postgres: tenant is required")
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	after, err := parseSeqCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	if err := scopeTenant(ctx, s.db, tenant); err != nil {
		return nil, "", err
	}

	var rows []models.EntryModel

	if err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND account_id = ? AND account_seq > ?", tenant.String(), account.String(), after).
		Order("account_seq").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, "", fmt.Errorf("postgres: list entries by account: %w", err)
	}

	next := ""

	if len(rows) > limit {
		next = strconv.FormatInt(rows[limit-1].AccountSeq, 10)
		rows = rows[:limit]
	}

	out := make([]entity.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryToEntity(row))
	}

	return out, next, nil
}

// Compile-time port assertion.
var _ repository.EntryReader = (*EntryStore)(nil)
