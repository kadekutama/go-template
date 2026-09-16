package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestOpsReads(t *testing.T) {
	t.Parallel()

	t.Run("run and break reads delegate to store", func(t *testing.T) {
		store := &opsReconStore{run: command.ReconRunRecord{ID: "run-1", Status: command.ReconRunPending}}
		svc := query.NewReconQueryService(query.ReconQueryServiceParams{Runs: store})
		run, err := svc.GetRun(context.Background(), "t-1", "run-1")
		require.NoError(t, err)
		assert.Equal(t, "run-1", run.ID)
	})

	t.Run("period read delegates to store", func(t *testing.T) {
		store := &opsPeriodStore{period: entity.PeriodData{ID: "2026-09", Status: entity.PeriodOpen}}
		svc := query.NewPeriodQueryService(query.PeriodQueryServiceParams{Periods: store})
		period, err := svc.GetPeriod(context.Background(), "t-1", "l-1", "2026-09")
		require.NoError(t, err)
		assert.Equal(t, entity.PeriodOpen, period.Status)
	})

	t.Run("report status maps record", func(t *testing.T) {
		store := &opsReportStore{record: command.ReportRecord{ID: "rep-1", Status: command.ReportReady, URL: "https://cdn.example/x"}}
		svc := query.NewReportQueryService(query.ReportQueryServiceParams{Reports: store})
		actualResult, err := svc.GetReport(context.Background(), "t-1", "rep-1")
		require.NoError(t, err)
		assert.Equal(t, port.ReportResult{ReportID: "rep-1", Status: command.ReportReady, SignedURL: "https://cdn.example/x"}, actualResult)
	})

	t.Run("export read delegates to store", func(t *testing.T) {
		store := &opsComplianceStore{export: command.RegulatoryExport{ID: "exp-1", ReportType: "CALL_REPORT"}}
		svc := query.NewComplianceQueryService(query.ComplianceQueryServiceParams{Reviews: store})
		export, err := svc.GetExport(context.Background(), "t-1", "exp-1")
		require.NoError(t, err)
		assert.Equal(t, "CALL_REPORT", export.ReportType)
	})
}

type opsReconStore struct {
	run command.ReconRunRecord
}

func (s *opsReconStore) CreateRun(_ context.Context, _ command.ReconRunRecord) error {
	return nil
}

func (s *opsReconStore) FindRun(_ context.Context, _ valueobject.TenantID, _ string) (command.ReconRunRecord, error) {
	return s.run, nil
}

func (s *opsReconStore) UpdateRun(_ context.Context, _ command.ReconRunRecord) error {
	return nil
}

func (s *opsReconStore) CreateBreaks(_ context.Context, _ []command.BreakRecord) error {
	return nil
}

func (s *opsReconStore) FindBreak(_ context.Context, _ valueobject.TenantID, _ string) (command.BreakRecord, error) {
	return command.BreakRecord{}, nil
}

func (s *opsReconStore) UpdateBreak(_ context.Context, _ command.BreakRecord) error {
	return nil
}

func (s *opsReconStore) CountOpenBreaks(_ context.Context, _ valueobject.TenantID, _ valueobject.LedgerID) (int, error) {
	return 0, nil
}

func (s *opsReconStore) ListBreaksByRun(_ context.Context, _ valueobject.TenantID, _ string) ([]command.BreakRecord, error) {
	return nil, nil
}

func (s *opsReconStore) ListRuns(_ context.Context, _ valueobject.TenantID, _ int) ([]command.ReconRunRecord, error) {
	return nil, nil
}

func (s *opsReconStore) ListBreaks(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]command.BreakRecord, error) {
	return nil, nil
}

type opsPeriodStore struct {
	period entity.PeriodData
}

func (s *opsPeriodStore) FindPeriod(_ context.Context, _, _, _ string) (entity.PeriodData, error) {
	return s.period, nil
}

func (s *opsPeriodStore) SavePeriod(_ context.Context, _ entity.PeriodData) error {
	return nil
}

func (s *opsPeriodStore) ListPeriods(_ context.Context, _, _ string, _ int) ([]entity.PeriodData, error) {
	return nil, nil
}

type opsReportStore struct {
	record command.ReportRecord
}

func (s *opsReportStore) CreateReport(_ context.Context, _ command.ReportRecord) error {
	return nil
}

func (s *opsReportStore) FindReport(_ context.Context, _ valueobject.TenantID, _ string) (command.ReportRecord, error) {
	return s.record, nil
}

func (s *opsReportStore) UpdateReport(_ context.Context, _ command.ReportRecord) error {
	return nil
}

func (s *opsReportStore) ListReports(_ context.Context, _ valueobject.TenantID, _ int) ([]command.ReportRecord, error) {
	return nil, nil
}

type opsComplianceStore struct {
	export command.RegulatoryExport
}

func (s *opsComplianceStore) RecordDecision(_ context.Context, _ command.ScreeningDecision) error {
	return nil
}

func (s *opsComplianceStore) SaveExport(_ context.Context, _ command.RegulatoryExport) error {
	return nil
}

func (s *opsComplianceStore) FindExport(_ context.Context, _ valueobject.TenantID, _ string) (command.RegulatoryExport, error) {
	return s.export, nil
}
