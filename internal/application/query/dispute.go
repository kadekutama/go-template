package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
)

// DisputeQueryServiceParams encapsulates dependencies for DisputeQueryService.
type DisputeQueryServiceParams struct {
	Disputes command.DisputeStore
}

// DisputeQueryService serves dispute reads for the edge by querying DisputeStore directly.
type DisputeQueryService struct {
	disputes command.DisputeStore
}

// NewDisputeQueryService creates an encapsulated DisputeQueryService with validated dependencies.
func NewDisputeQueryService(params DisputeQueryServiceParams) *DisputeQueryService {
	return &DisputeQueryService{
		disputes: params.Disputes,
	}
}

var _ port.DisputeQueryUseCases = (*DisputeQueryService)(nil)

// GetDispute returns one dispute with deadline + fee. Strong read.
func (s *DisputeQueryService) GetDispute(ctx context.Context, query port.DisputeQuery) (port.DisputeResult, error) {
	dispute, err := s.disputes.FindDispute(ctx, query.DisputeID)
	if err != nil {
		return port.DisputeResult{}, err
	}
	return port.DisputeResult{Dispute: dispute}, nil
}

// ListDisputes pages disputes by filter. Point-in-time page.
func (s *DisputeQueryService) ListDisputes(ctx context.Context, filter port.DisputeListFilter) (port.DisputePage, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}
	disputes, next, err := s.disputes.ListDisputes(ctx, filter.Status, filter.From, filter.To, filter.Cursor, limit)
	if err != nil {
		return port.DisputePage{}, err
	}
	results := make([]port.DisputeResult, 0, len(disputes))
	for _, dispute := range disputes {
		results = append(results, port.DisputeResult{Dispute: dispute})
	}
	return port.DisputePage{Disputes: results, NextCursor: next}, nil
}
