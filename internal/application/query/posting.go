// Package query owns the core read use cases: strong posting and balance
// reads plus cursor-paged entry lists. Queries never perform business side
// effects; spend decisions use these strong reads, never cached figures.
package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/repository"
)

// PostingQueryServiceParams encapsulates dependencies for PostingQueryService.
type PostingQueryServiceParams struct {
	Postings repository.PostingRepository
}

// PostingQueryService answers single-posting strong reads.
type PostingQueryService struct {
	postings repository.PostingRepository
}

// NewPostingQueryService creates an encapsulated PostingQueryService with validated dependencies.
func NewPostingQueryService(params PostingQueryServiceParams) *PostingQueryService {
	return &PostingQueryService{
		postings: params.Postings,
	}
}

var _ port.GetPosting = (*PostingQueryService)(nil)

// Execute returns one committed posting with entries. Strong read; Cursor is
// empty because point reads carry no page position.
func (s *PostingQueryService) Execute(ctx context.Context, query port.GetPostingQuery) (port.PostingView, error) {
	posting, err := s.postings.FindByID(ctx, query.TenantID, query.PostingID)
	if err != nil {
		return port.PostingView{}, err
	}
	return port.PostingView{Posting: posting}, nil
}
