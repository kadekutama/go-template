package query

import (
	"context"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
)

// Page limit bounds for entries: at least one row, at most one bounded scan per call.
const (
	entriesMinLimit = 1
	entriesMaxLimit = 500
)

// EntriesServiceParams encapsulates dependencies for EntriesService.
type EntriesServiceParams struct {
	Entries repository.EntryReader
}

// EntriesService pages committed journal lines for one account in cursor
// order. Pages are point-in-time; spend decisions use the strong reads
// (GetPosting/GetBalance) instead.
type EntriesService struct {
	entries repository.EntryReader
}

// NewEntriesService creates an encapsulated EntriesService with validated dependencies.
func NewEntriesService(params EntriesServiceParams) *EntriesService {
	return &EntriesService{
		entries: params.Entries,
	}
}

var _ port.ListEntries = (*EntriesService)(nil)

// Execute returns one page of entries with the adapter's next cursor.
func (s *EntriesService) Execute(ctx context.Context, query port.EntriesQuery) (port.EntriesPage, error) {
	if err := validateEntriesQuery(query); err != nil {
		return port.EntriesPage{}, err
	}
	entries, next, err := s.entries.FindByAccount(ctx, query.TenantID, query.AccountID, query.Cursor, query.Limit)
	if err != nil {
		return port.EntriesPage{}, err
	}
	return port.EntriesPage{Entries: entries, NextCursor: next}, nil
}

// validateEntriesQuery checks the query envelope.
func validateEntriesQuery(query port.EntriesQuery) error {
	if strings.TrimSpace(query.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(query.AccountID.String()) == "" {
		return entity.NewError("ENTRIES_ACCOUNT_REQUIRED", "entries require an account id")
	}
	if query.Limit < entriesMinLimit || query.Limit > entriesMaxLimit {
		return entity.NewError("INVALID_EXPORT_LIMIT", "export limit must be between 1 and 500")
	}
	return nil
}
