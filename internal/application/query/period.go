package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// PeriodQueryServiceParams encapsulates dependencies for PeriodQueryService.
type PeriodQueryServiceParams struct {
	Periods command.PeriodStore
}

// PeriodQueryService serves period reads for the edge. Strong reads.
type PeriodQueryService struct {
	periods command.PeriodStore
}

// NewPeriodQueryService creates an encapsulated PeriodQueryService with validated dependencies.
func NewPeriodQueryService(params PeriodQueryServiceParams) *PeriodQueryService {
	return &PeriodQueryService{
		periods: params.Periods,
	}
}

// GetPeriod returns one accounting period. Strong read.
func (s *PeriodQueryService) GetPeriod(ctx context.Context, tenant, ledger, id string) (entity.PeriodData, error) {
	return s.periods.FindPeriod(ctx, tenant, ledger, id)
}

// ListPeriods returns one tenant/ledger's periods, newest first (bounded). Strong read.
func (s *PeriodQueryService) ListPeriods(ctx context.Context, tenant, ledger string, limit int) ([]entity.PeriodData, error) {
	return s.periods.ListPeriods(ctx, tenant, ledger, limit)
}
