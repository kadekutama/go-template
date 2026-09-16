package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ReconQueryServiceParams encapsulates dependencies for ReconQueryService.
type ReconQueryServiceParams struct {
	Runs command.ReconStore
}

// ReconQueryService serves reconciliation reads for the edge. Strong reads
// for runs; break pages are point-in-time.
type ReconQueryService struct {
	runs command.ReconStore
}

// NewReconQueryService creates an encapsulated ReconQueryService with validated dependencies.
func NewReconQueryService(params ReconQueryServiceParams) *ReconQueryService {
	return &ReconQueryService{
		runs: params.Runs,
	}
}

// GetRun returns one reconciliation run. Strong read.
func (s *ReconQueryService) GetRun(ctx context.Context, tenant valueobject.TenantID, runID string) (command.ReconRunRecord, error) {
	return s.runs.FindRun(ctx, tenant, runID)
}

// GetBreak returns one break record. Strong read.
func (s *ReconQueryService) GetBreak(ctx context.Context, tenant valueobject.TenantID, breakID string) (command.BreakRecord, error) {
	return s.runs.FindBreak(ctx, tenant, breakID)
}

// ListRuns returns one tenant's runs, newest first (bounded). Strong read.
func (s *ReconQueryService) ListRuns(ctx context.Context, tenant valueobject.TenantID, limit int) ([]command.ReconRunRecord, error) {
	return s.runs.ListRuns(ctx, tenant, limit)
}

// ListBreaks returns one tenant's breaks by status (empty = all), bounded. Strong read.
func (s *ReconQueryService) ListBreaks(ctx context.Context, tenant valueobject.TenantID, status string, limit int) ([]command.BreakRecord, error) {
	return s.runs.ListBreaks(ctx, tenant, status, limit)
}
