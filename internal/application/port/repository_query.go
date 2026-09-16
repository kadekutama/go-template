package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PostingFilter searches committed postings with optional bounds. All bounds
// are tenant-scoped; adapters MUST NOT return cross-tenant rows.
type PostingFilter struct {
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	AccountID valueobject.AccountID
	Operation string
	Status    string
	MinAmount int64
	MaxAmount int64
	From      time.Time
	To        time.Time
	Cursor    string
	Limit     int
}

// PostingSearchPage is one point-in-time page of committed postings.
type PostingSearchPage struct {
	Postings   []entity.PostingData
	NextCursor string
}

// PostingQuery is the filtered read-model search over committed postings,
// backing reports, dashboards, and operational search. It never substitutes
// for strong reads in spend decisions.
type PostingQuery interface {
	// Search returns one page of committed postings by filter. Point-in-time page.
	Search(ctx context.Context, filter PostingFilter) (PostingSearchPage, error)
}
