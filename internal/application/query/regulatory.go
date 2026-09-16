package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RegulatoryQueryServiceParams encapsulates dependencies for RegulatoryQueryService.
type RegulatoryQueryServiceParams struct {
	Reviews command.ComplianceStore
}

// RegulatoryQueryService serves regulatory export reads plus the supported
// type catalog. Single-type rendering stays with the T05 compliance service;
// this surface answers what exists and what is supported.
type RegulatoryQueryService struct {
	reviews command.ComplianceStore
}

// NewRegulatoryQueryService creates an encapsulated RegulatoryQueryService with validated dependencies.
func NewRegulatoryQueryService(params RegulatoryQueryServiceParams) *RegulatoryQueryService {
	return &RegulatoryQueryService{
		reviews: params.Reviews,
	}
}

// GetExport returns one rendered regulatory file. Strong read.
func (s *RegulatoryQueryService) GetExport(ctx context.Context, tenant valueobject.TenantID, exportID string) (command.RegulatoryExport, error) {
	return s.reviews.FindExport(ctx, tenant, exportID)
}

// SupportedTypes lists the regulatory report types with field definitions.
func (s *RegulatoryQueryService) SupportedTypes() []service.ReportType {
	return []service.ReportType{
		service.ReportCallReport,
		service.ReportForm1099,
		service.ReportFATCA,
		service.ReportCRS,
	}
}
