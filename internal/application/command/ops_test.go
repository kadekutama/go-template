package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func newPeriodService(t *testing.T, uow port.UnitOfWork, periods command.PeriodStore, breaks command.ReconStore, authz port.Authorizer) *command.PeriodService {
	return command.NewPeriodService(command.PeriodServiceParams{
		UoW: uow, Periods: periods, Breaks: breaks, Clock: newMockClock(t, tfrAt),
		IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
	})
}

func periodTestRequest() port.PeriodRequest {
	return port.PeriodRequest{
		TenantID: tfrTenant, LedgerID: tfrLedger, PeriodID: "2026-09",
		Start: opsAt.Add(-30 * 24 * time.Hour), End: opsAt, Timezone: "UTC",
		Approver: "u-2", Actor: "u-1",
		UnresolvedWorkflows: 0, SubledgerDeltas: map[string]int64{}, FXRevalued: true,
	}
}

func TestPeriodOpen(t *testing.T) {
	t.Parallel()

	baseReq := periodTestRequest()

	type testCase struct {
		name          string
		req           port.PeriodRequest
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid open creates period with fact",
			req:           baseReq,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.PeriodRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger returns LEDGER_REQUIRED",
			req: func() port.PeriodRequest {
				r := baseReq
				r.LedgerID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "missing period id returns PERIOD_ID_REQUIRED",
			req: func() port.PeriodRequest {
				r := baseReq
				r.PeriodID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("PERIOD_ID_REQUIRED", "period id is required"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.PeriodRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "period actor is required"),
		},
		{
			name: "zero start returns PERIOD_BOUNDS_INVALID",
			req: func() port.PeriodRequest {
				r := baseReq
				r.Start = time.Time{}
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("PERIOD_BOUNDS_INVALID", "period start must precede end"),
		},
		{
			name: "end before start returns PERIOD_BOUNDS_INVALID",
			req: func() port.PeriodRequest {
				r := baseReq
				r.End = r.Start.Add(-time.Hour)
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("PERIOD_BOUNDS_INVALID", "period start must precede end"),
		},
		{
			name: "missing timezone returns PERIOD_TIMEZONE_REQUIRED",
			req: func() port.PeriodRequest {
				r := baseReq
				r.Timezone = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("PERIOD_TIMEZONE_REQUIRED", "accounting timezone is required"),
		},
		{
			name:          "denied subject returns error",
			req:           baseReq,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			periods := newMockPeriodStore(t)
			breaks := newMockReconStore(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|period.open|ledger/"+string(tc.req.LedgerID))
			}
			svc := newPeriodService(t, uow, periods, breaks, authz)

			err := svc.OpenPeriod(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				replayErr := svc.OpenPeriod(context.Background(), tc.req)
				assert.NoError(t, replayErr)
			}
		})
	}
}

func TestPeriodClose(t *testing.T) {
	t.Parallel()

	baseReq := periodTestRequest()

	type testCase struct {
		name          string
		req           port.PeriodRequest
		preload       func(periods command.PeriodStore)
		openBreaks    int
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid close succeeds with outbox fact",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodOpen,
				})
			},
			openBreaks:    0,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "period not found returns error",
			req:  baseReq,
			preload: func(_ command.PeriodStore) {
			},
			openBreaks:    0,
			denied:        false,
			expectedError: entity.NewError("PERIOD_NOT_FOUND", "period is unknown"),
		},
		{
			name: "already closed period rejected",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodClosed,
				})
			},
			openBreaks:    0,
			denied:        false,
			expectedError: entity.NewError("PERIOD_CLOSED", "period close requires an open period"),
		},
		{
			name: "unbalanced subledgers rejected",
			req: func() port.PeriodRequest {
				r := baseReq
				r.SubledgerDeltas = map[string]int64{"sub-1": 100}
				return r
			}(),
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodOpen,
				})
			},
			openBreaks:    0,
			denied:        false,
			expectedError: entity.NewError("SUBLEDGERS_UNBALANCED", "sub-ledgers do not balance"),
		},
		{
			name: "dirty close reports every failing code",
			req: func() port.PeriodRequest {
				r := baseReq
				r.UnresolvedWorkflows = 3
				r.FXRevalued = false
				return r
			}(),
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodOpen,
				})
			},
			openBreaks:    2,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "denied subject fails closed",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodOpen,
				})
			},
			openBreaks:    0,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			periods := newMockPeriodStore(t)
			tc.preload(periods)
			breaks := newMockReconStoreWithOpenBreaks(t, tc.openBreaks)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|period.close|ledger/"+string(tc.req.LedgerID))
			}
			svc := newPeriodService(t, uow, periods, breaks, authz)

			err := svc.ClosePeriod(context.Background(), tc.req)
			if tc.name == "dirty close reports every failing code" {
				require.Error(t, err)
				for _, code := range []string{"WORKFLOWS_PENDING", "BREAKS_OPEN", "FX_NOT_REVALUED"} {
					assert.Contains(t, err.Error(), code)
				}
				return
			}
			if tc.expectedError != nil && (tc.name == "already closed period rejected" || tc.name == "unbalanced subledgers rejected") {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
				return
			}
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				replayErr := svc.ClosePeriod(context.Background(), tc.req)
				assert.NoError(t, replayErr)
			}
		})
	}
}

func TestPeriodReopen(t *testing.T) {
	t.Parallel()

	baseReq := periodTestRequest()

	type testCase struct {
		name          string
		req           port.PeriodRequest
		preload       func(periods command.PeriodStore)
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid reopen restores period open with fact",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodClosed,
				})
			},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "open period cannot reopen",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodOpen,
				})
			},
			denied:        false,
			expectedError: entity.NewError("PERIOD_STATE_INVALID", "only closed periods can reopen"),
		},
		{
			name: "self approval rejected by SoD",
			req: func() port.PeriodRequest {
				r := baseReq
				r.Approver = r.Actor
				return r
			}(),
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodClosed,
				})
			},
			denied:        false,
			expectedError: entity.NewError("SELF_APPROVAL_FORBIDDEN", "period reopen checker must differ from maker"),
		},
		{
			name: "period not found propagates error",
			req:  baseReq,
			preload: func(_ command.PeriodStore) {
			},
			denied:        false,
			expectedError: entity.NewError("PERIOD_NOT_FOUND", "period is unknown"),
		},
		{
			name: "denied subject fails closed",
			req:  baseReq,
			preload: func(periods command.PeriodStore) {
				_ = periods.SavePeriod(context.Background(), entity.PeriodData{
					ID: "2026-09", TenantID: "t-1", LedgerID: "l-1", Status: entity.PeriodClosed,
				})
			},
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			periods := newMockPeriodStore(t)
			tc.preload(periods)
			breaks := newMockReconStore(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|period.reopen|ledger/"+string(tc.req.LedgerID))
			}
			svc := newPeriodService(t, uow, periods, breaks, authz)

			err := svc.ReopenPeriod(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				replayErr := svc.ReopenPeriod(context.Background(), tc.req)
				assert.NoError(t, replayErr)
			}
		})
	}
}

func TestReportGenerate(t *testing.T) {
	t.Parallel()

	baseReq := port.ReportRequest{
		TenantID:       tfrTenant,
		Template:       "balance-sheet",
		Parameters:     map[string]string{"period": "2026-09"},
		Destination:    "cdn",
		IdempotencyKey: "key-report-1",
		Actor:          "u-1",
	}

	type testCase struct {
		name          string
		req           port.ReportRequest
		preload       func(uow port.UnitOfWork)
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "generates balance sheet report",
			req:           baseReq,
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  baseReq,
			preload: func(uow port.UnitOfWork) {
				parts := append([]string{baseReq.IdempotencyKey, string(baseReq.TenantID), baseReq.Template, baseReq.Destination},
					command.MapParts("params", baseReq.Parameters)...)
				fp := command.Fingerprint(parts...)
				setUOWIdem(uow, baseReq.IdempotencyKey, fp, []byte("{corrupt-json"))
			},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "generates income statement report",
			req: func() port.ReportRequest {
				r := baseReq
				r.Template = "income-statement"
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing tenant rejected",
			req: func() port.ReportRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "unknown template rejected",
			req: func() port.ReportRequest {
				r := baseReq
				r.Template = "fortune-cookie"
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("REPORT_TEMPLATE_UNKNOWN", "report template is unknown"),
		},
		{
			name: "missing actor rejected",
			req: func() port.ReportRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "report actor is required"),
		},
		{
			name: "missing idempotency key rejected",
			req: func() port.ReportRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "report requires an idempotency key"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			req:           baseReq,
			preload:       func(_ port.UnitOfWork) {},
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			reports := newMockReportStore(t)
			storage := newMockObjectStorage(t)
			tc.preload(uow)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|report.generate|tenant/"+string(tc.req.TenantID))
			}
			svc := command.NewReportService(command.ReportServiceParams{
				UoW: uow, Reports: reports, Storage: storage, Clock: newMockClock(t, tfrAt),
				IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
			})
			actualResult, err := svc.GenerateReport(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError != nil {
				return
			}
			assert.Equal(t, command.ReportReady, actualResult.Status)
			assert.Contains(t, actualResult.SignedURL, "https://cdn.example/")
			assert.Contains(t, outboxTypes(uow), command.EventReportGenerated)
			stored, findErr := reports.FindReport(context.Background(), tfrTenant, actualResult.ReportID)
			require.NoError(t, findErr)
			assert.Equal(t, tc.req.Template, stored.Template)

			// Idempotency replay
			replayResult, replayErr := svc.GenerateReport(context.Background(), tc.req)
			assert.NoError(t, replayErr)
			assert.Equal(t, actualResult.ReportID, replayResult.ReportID)
		})
	}
}

func TestReportStatusAndListTemplates(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		reportID      string
		seedReport    bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "get existing report status returns result",
			reportID:      "rep-01",
			seedReport:    true,
			expectedError: nil,
		},
		{
			name:          "get missing report status returns error",
			reportID:      "rep-missing",
			seedReport:    false,
			expectedError: entity.NewError("REPORT_NOT_FOUND", "report is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reports := newMockReportStore(t)
			storage := newMockObjectStorage(t)
			svc := command.NewReportService(command.ReportServiceParams{
				UoW: newMockUOW(t), Reports: reports, Storage: storage, Clock: newMockClock(t, tfrAt),
				IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: newMockAuthorizer(t),
			})

			if tc.seedReport {
				_ = reports.CreateReport(context.Background(), command.ReportRecord{
					ID:       tc.reportID,
					TenantID: tfrTenant,
					Template: "balance-sheet",
					Status:   command.ReportReady,
					URL:      "https://cdn.example/rep-01",
				})
			}

			res, err := svc.ReportStatus(context.Background(), tfrTenant, tc.reportID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.reportID, res.ReportID)
				assert.Equal(t, command.ReportReady, res.Status)
			}
		})
	}

	// Verify ListTemplates returns all known report templates
	svc := command.NewReportService(command.ReportServiceParams{})
	templates := svc.ListTemplates()
	assert.Equal(t, command.ReportTemplates, templates)
}

func TestComplianceRecordScreeningDecision(t *testing.T) {
	t.Parallel()

	baseReq := port.ScreeningRequest{
		TenantID:       tfrTenant,
		SubjectID:      "party-1",
		Decision:       "ALLOW",
		Note:           "clear",
		Actor:          "u-1",
		IdempotencyKey: "key-screen-1",
	}

	type testCase struct {
		name          string
		req           port.ScreeningRequest
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid decision records auditably",
			req:           baseReq,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.ScreeningRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing subject returns SCREENING_SUBJECT_REQUIRED",
			req: func() port.ScreeningRequest {
				r := baseReq
				r.SubjectID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("SCREENING_SUBJECT_REQUIRED", "screening subject is required"),
		},
		{
			name: "missing decision returns SCREENING_DECISION_REQUIRED",
			req: func() port.ScreeningRequest {
				r := baseReq
				r.Decision = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("SCREENING_DECISION_REQUIRED", "screening decision is required"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.ScreeningRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "compliance actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.ScreeningRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "compliance requires an idempotency key"),
		},
		{
			name:          "denied subject returns error",
			req:           baseReq,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockComplianceStore(t)
			storage := newMockObjectStorage(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|compliance.review|tenant/"+string(tc.req.TenantID))
			}
			svc := command.NewComplianceService(command.ComplianceServiceParams{
				UoW: uow, Reviews: store, Storage: storage, Clock: newMockClock(t, tfrAt),
				IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
			})

			err := svc.RecordScreeningDecision(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				replayErr := svc.RecordScreeningDecision(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				store.AssertNumberOfCalls(t, "RecordDecision", 1)
			}
		})
	}
}

func TestComplianceExportRegulatoryReport(t *testing.T) {
	t.Parallel()

	baseReq := port.RegulatoryExportRequest{
		TenantID:   tfrTenant,
		ReportType: "CALL_REPORT",
		PeriodID:   "2026-09",
		Fields: map[string]string{
			"tenant_id": "t-1", "period_start": "2026-09-01", "period_end": "2026-09-30",
			"total_assets_minor": "100000", "total_liabilities_minor": "90000", "rule_version": "v1",
		},
		Actor:          "u-1",
		IdempotencyKey: "key-export-1",
	}

	type testCase struct {
		name          string
		req           port.RegulatoryExportRequest
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid regulatory export succeeds",
			req:           baseReq,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.RegulatoryExportRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.RegulatoryExportRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "compliance actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.RegulatoryExportRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "compliance requires an idempotency key"),
		},
		{
			name: "unknown regulatory type rejected",
			req: func() port.RegulatoryExportRequest {
				r := baseReq
				r.ReportType = "FORM_ZZZ"
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
		{
			name: "missing regulatory field rejected",
			req: func() port.RegulatoryExportRequest {
				r := baseReq
				fields := make(map[string]string, len(baseReq.Fields))
				for k, v := range baseReq.Fields {
					fields[k] = v
				}
				delete(fields, "period_start")
				r.Fields = fields
				return r
			}(),
			denied:        false,
			expectedError: entity.NewError("REPORT_FIELD_MISSING", "report field period_start is required"),
		},
		{
			name:          "denied subject returns error",
			req:           baseReq,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockComplianceStore(t)
			storage := newMockObjectStorage(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|compliance.export|tenant/"+string(tc.req.TenantID))
			}
			svc := command.NewComplianceService(command.ComplianceServiceParams{
				UoW: uow, Reviews: store, Storage: storage, Clock: newMockClock(t, tfrAt),
				IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
			})

			result, err := svc.ExportRegulatoryReport(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, command.ReportReady, result.Status)
				replayResult, replayErr := svc.ExportRegulatoryReport(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				assert.Equal(t, result.ReportID, replayResult.ReportID)
			}
		})
	}
}
