package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RunSummary aggregates one reconciliation run with per-status break counts.
type RunSummary struct {
	RunID    string
	Status   string
	Total    int
	ByStatus map[string]int
}

// ReconReportServiceParams encapsulates dependencies for ReconReportService.
type ReconReportServiceParams struct {
	Runs command.ReconStore
}

// ReconReportService serves reconciliation reports: run summaries and
// status-filtered break lists over a run.
type ReconReportService struct {
	runs command.ReconStore
}

// NewReconReportService creates an encapsulated ReconReportService with validated dependencies.
func NewReconReportService(params ReconReportServiceParams) *ReconReportService {
	return &ReconReportService{
		runs: params.Runs,
	}
}

// SummarizeRun aggregates totals + by-status counts for one run. Strong read.
func (s *ReconReportService) SummarizeRun(ctx context.Context, tenant valueobject.TenantID, runID string) (RunSummary, error) {
	run, err := s.runs.FindRun(ctx, tenant, runID)
	if err != nil {
		return RunSummary{}, err
	}
	breaks, err := s.runs.ListBreaksByRun(ctx, tenant, runID)
	if err != nil {
		return RunSummary{}, err
	}
	summary := RunSummary{RunID: run.ID, Status: run.Status, ByStatus: map[string]int{}}
	for _, record := range breaks {
		status := string(record.Status)
		summary.ByStatus[status]++
		summary.Total++
	}
	return summary, nil
}

// BreaksByStatus lists one run's breaks filtered by status. Point-in-time read.
func (s *ReconReportService) BreaksByStatus(ctx context.Context, tenant valueobject.TenantID, runID, status string, limit int) ([]command.BreakRecord, error) {
	breaks, err := s.runs.ListBreaksByRun(ctx, tenant, runID)
	if err != nil {
		return nil, err
	}
	filtered := make([]command.BreakRecord, 0, len(breaks))
	for _, record := range breaks {
		if status != "" && string(record.Status) != status {
			continue
		}
		filtered = append(filtered, record)
		if limit > 0 && len(filtered) >= limit {
			break
		}
	}
	return filtered, nil
}
