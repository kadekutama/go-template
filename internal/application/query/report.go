package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ReportQueryService serves report reads for the edge. Strong reads.
// ReportQueryServiceParams encapsulates dependencies for ReportQueryService.
type ReportQueryServiceParams struct {
	Reports command.ReportStore
}

// ReportQueryService serves report reads for the edge. Strong reads.
type ReportQueryService struct {
	reports command.ReportStore
}

// NewReportQueryService creates an encapsulated ReportQueryService with validated dependencies.
func NewReportQueryService(params ReportQueryServiceParams) *ReportQueryService {
	return &ReportQueryService{
		reports: params.Reports,
	}
}

// GetReport returns one report's generation state. Strong read.
func (s *ReportQueryService) GetReport(ctx context.Context, tenant valueobject.TenantID, reportID string) (port.ReportResult, error) {
	record, err := s.reports.FindReport(ctx, tenant, reportID)
	if err != nil {
		return port.ReportResult{}, err
	}
	return port.ReportResult{ReportID: record.ID, Status: record.Status, SignedURL: record.URL}, nil
}

// ListReports returns one tenant's reports, newest first (bounded). Strong read.
func (s *ReportQueryService) ListReports(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.ReportResult, error) {
	records, err := s.reports.ListReports(ctx, tenant, limit)
	if err != nil {
		return nil, err
	}
	out := make([]port.ReportResult, 0, len(records))
	for _, record := range records {
		out = append(out, port.ReportResult{ReportID: record.ID, Status: record.Status, SignedURL: record.URL})
	}
	return out, nil
}
