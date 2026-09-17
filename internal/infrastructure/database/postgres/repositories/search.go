package repositories

import (
	"context"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// PostingSearchParams carries constructor dependencies.
type PostingSearchParams struct {
	DB *gorm.DB
}

// PostingSearch is the PostgreSQL appport.PostingQuery adapter: filtered
// read-model search over committed postings. It never substitutes for strong
// reads in spend decisions.
type PostingSearch struct {
	db *gorm.DB
}

// NewPostingSearch builds the adapter; DB must be non-nil.
func NewPostingSearch(params PostingSearchParams) (*PostingSearch, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: posting search needs DB")
	}

	return &PostingSearch{db: params.DB}, nil
}

// Search returns one point-in-time page of committed postings by filter.
// Amount bounds and workflow status filters are rejected explicitly: amounts
// aggregate per posting in the read model (E06 reports), and postings carry
// no lifecycle status (corrections are new linked postings).
func (s *PostingSearch) Search(ctx context.Context, filter appport.PostingFilter) (appport.PostingSearchPage, error) {
	limit, after, err := validateSearch(filter)
	if err != nil {
		return appport.PostingSearchPage{}, err
	}

	if err := scopeTenant(ctx, s.db, filter.TenantID); err != nil {
		return appport.PostingSearchPage{}, err
	}

	query := s.db.WithContext(ctx).Model(&models.PostingModel{}).
		Where("tenant_id = ? AND ledger_seq > ?", filter.TenantID.String(), after)

	query = applySearchFilters(query, filter)

	if filter.AccountID.String() != "" {
		ids, err := s.postingsForAccount(ctx, filter)
		if err != nil {
			return appport.PostingSearchPage{}, err
		}

		if len(ids) == 0 {
			return appport.PostingSearchPage{Postings: []entity.PostingData{}}, nil
		}

		query = query.Where("id IN ?", ids)
	}

	var postingRows []models.PostingModel

	if err := query.Order("ledger_seq").Limit(limit + 1).Find(&postingRows).Error; err != nil {
		return appport.PostingSearchPage{}, fmt.Errorf("postgres: search postings: %w", err)
	}

	next := ""

	if len(postingRows) > limit {
		next = strconv.FormatInt(postingRows[limit-1].LedgerSeq, 10)
		postingRows = postingRows[:limit]
	}

	if len(postingRows) == 0 {
		return appport.PostingSearchPage{Postings: []entity.PostingData{}}, nil
	}

	ids := make([]string, 0, len(postingRows))
	for _, row := range postingRows {
		ids = append(ids, row.ID)
	}

	out, err := assembleSearchEntries(ctx, s.db, filter.TenantID, postingRows, ids)
	if err != nil {
		return appport.PostingSearchPage{}, err
	}

	return appport.PostingSearchPage{Postings: out, NextCursor: next}, nil
}

// validateSearch rejects unsupported filters and normalizes paging.
func validateSearch(filter appport.PostingFilter) (int, int64, error) {
	if filter.TenantID.String() == "" {
		return 0, 0, fmt.Errorf("postgres: tenant is required")
	}

	if filter.MinAmount != 0 || filter.MaxAmount != 0 {
		return 0, 0, fmt.Errorf("postgres: amount bounds need the reporting read model (E06-T09)")
	}

	if filter.Status != "" {
		return 0, 0, fmt.Errorf("postgres: postings carry no status; corrections are new linked postings")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	after, err := parseSeqCursor(filter.Cursor)
	if err != nil {
		return 0, 0, err
	}

	return limit, after, nil
}

// applySearchFilters adds ledger/operation/time predicates.
func applySearchFilters(query *gorm.DB, filter appport.PostingFilter) *gorm.DB {
	if filter.LedgerID.String() != "" {
		query = query.Where("ledger_id = ?", filter.LedgerID.String())
	}

	if filter.Operation != "" {
		query = query.Where("operation = ?", filter.Operation)
	}

	if !filter.From.IsZero() {
		query = query.Where("effective_at >= ?", filter.From)
	}

	if !filter.To.IsZero() {
		query = query.Where("effective_at <= ?", filter.To)
	}

	return query
}

// postingsForAccount resolves posting IDs touching an account.
func (s *PostingSearch) postingsForAccount(ctx context.Context, filter appport.PostingFilter) ([]string, error) {
	var ids []string

	if err := s.db.WithContext(ctx).Model(&models.EntryModel{}).
		Where("tenant_id = ? AND account_id = ?", filter.TenantID.String(), filter.AccountID.String()).
		Pluck("posting_id", &ids).Error; err != nil {
		return nil, fmt.Errorf("postgres: search account scope: %w", err)
	}

	return ids, nil
}

// assembleSearchEntries batch-fetches entries for found postings.
func assembleSearchEntries(
	ctx context.Context,
	db *gorm.DB,
	tenant valueobject.TenantID,
	postingRows []models.PostingModel,
	ids []string,
) ([]entity.PostingData, error) {
	var entryRows []models.EntryModel

	if err := db.WithContext(ctx).Where("tenant_id = ? AND posting_id IN ?", tenant.String(), ids).
		Order("account_seq").Find(&entryRows).Error; err != nil {
		return nil, fmt.Errorf("postgres: search entries: %w", err)
	}

	byPosting := make(map[string][]entity.Entry, len(ids))
	for _, row := range entryRows {
		byPosting[row.PostingID] = append(byPosting[row.PostingID], entryToEntity(row))
	}

	out := make([]entity.PostingData, 0, len(postingRows))

	for _, row := range postingRows {
		out = append(out, postingToEntity(row, byPosting[row.ID]))
	}

	return out, nil
}

// Compile-time assertion: the application read-model port.
var _ appport.PostingQuery = (*PostingSearch)(nil)
