package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
)

func TestOpsReads(t *testing.T) {
	t.Parallel()

	t.Run("run and break reads delegate to store", func(t *testing.T) {
		store := mockcommand.NewMockReconStore(t)
		store.EXPECT().
			FindRun(mock.Anything, valueobject.TenantID("t-1"), "run-1").
			Return(command.ReconRunRecord{ID: "run-1", Status: command.ReconRunPending}, nil).
			Once()

		svc := query.NewReconQueryService(query.ReconQueryServiceParams{Runs: store})
		run, err := svc.GetRun(context.Background(), "t-1", "run-1")
		require.NoError(t, err)
		assert.Equal(t, "run-1", run.ID)
	})

	t.Run("period read delegates to store", func(t *testing.T) {
		store := mockcommand.NewMockPeriodStore(t)
		store.EXPECT().
			FindPeriod(mock.Anything, "t-1", "l-1", "2026-09").
			Return(entity.PeriodData{ID: "2026-09", Status: entity.PeriodOpen}, nil).
			Once()

		svc := query.NewPeriodQueryService(query.PeriodQueryServiceParams{Periods: store})
		period, err := svc.GetPeriod(context.Background(), "t-1", "l-1", "2026-09")
		require.NoError(t, err)
		assert.Equal(t, entity.PeriodOpen, period.Status)
	})

	t.Run("report status maps record", func(t *testing.T) {
		store := mockcommand.NewMockReportStore(t)
		store.EXPECT().
			FindReport(mock.Anything, valueobject.TenantID("t-1"), "rep-1").
			Return(command.ReportRecord{ID: "rep-1", Status: command.ReportReady, URL: "https://cdn.example/x"}, nil).
			Once()

		svc := query.NewReportQueryService(query.ReportQueryServiceParams{Reports: store})
		actualResult, err := svc.GetReport(context.Background(), "t-1", "rep-1")
		require.NoError(t, err)
		assert.Equal(t, port.ReportResult{ReportID: "rep-1", Status: command.ReportReady, SignedURL: "https://cdn.example/x"}, actualResult)
	})

	t.Run("export read delegates to store", func(t *testing.T) {
		store := mockcommand.NewMockComplianceStore(t)
		store.EXPECT().
			FindExport(mock.Anything, valueobject.TenantID("t-1"), "exp-1").
			Return(command.RegulatoryExport{ID: "exp-1", ReportType: "CALL_REPORT"}, nil).
			Once()

		svc := query.NewComplianceQueryService(query.ComplianceQueryServiceParams{Reviews: store})
		export, err := svc.GetExport(context.Background(), "t-1", "exp-1")
		require.NoError(t, err)
		assert.Equal(t, "CALL_REPORT", export.ReportType)
	})
}
