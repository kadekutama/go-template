package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ComplianceQueryServiceParams encapsulates dependencies for ComplianceQueryService.
type ComplianceQueryServiceParams struct {
	Reviews command.ComplianceStore
}

// ComplianceQueryService serves compliance reads for the edge. Strong reads.
type ComplianceQueryService struct {
	reviews command.ComplianceStore
}

// NewComplianceQueryService creates an encapsulated ComplianceQueryService with validated dependencies.
func NewComplianceQueryService(params ComplianceQueryServiceParams) *ComplianceQueryService {
	return &ComplianceQueryService{
		reviews: params.Reviews,
	}
}

// GetExport returns one rendered regulatory file. Strong read.
func (s *ComplianceQueryService) GetExport(ctx context.Context, tenant valueobject.TenantID, exportID string) (command.RegulatoryExport, error) {
	return s.reviews.FindExport(ctx, tenant, exportID)
}
