package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ReconRunRequest triggers one reconciliation run over a statement window.
type ReconRunRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	Source         string
	WindowStart    time.Time
	WindowEnd      time.Time
	IdempotencyKey string
	Actor          string
}

// ReconRunResult identifies the run for break inspection.
type ReconRunResult struct {
	RunID  string
	Status string
	Cursor string
}

// BreakResolutionRequest resolves or acknowledges one break. Resolutions above
// the SoD threshold require a different approver; the rule is surfaced here,
// enforced by the domain.
type BreakResolutionRequest struct {
	TenantID            valueobject.TenantID
	BreakID             string
	Approver            string
	Decision            string
	Note                string
	Actor               string
	AdjustmentPostingID string
	IdempotencyKey      string
}

// PeriodRequest opens, closes, or reopens one accounting period. Close runs
// period validation and returns ALL failures, not first-only.
type PeriodRequest struct {
	TenantID            valueobject.TenantID
	LedgerID            valueobject.LedgerID
	PeriodID            valueobject.PeriodID
	Start               time.Time
	End                 time.Time
	Timezone            string
	Approver            string
	Actor               string
	UnresolvedWorkflows int
	SubledgerDeltas     map[string]int64
	FXRevalued          bool
}

// ReportRequest generates one templated report with delivery to a signed URL
// plus a report.generated event.
type ReportRequest struct {
	TenantID       valueobject.TenantID
	Template       string
	Parameters     map[string]string
	Destination    string
	IdempotencyKey string
	Actor          string
}

// ReportResult tracks generation through delivery.
type ReportResult struct {
	ReportID  string
	Status    string
	SignedURL string
	Cursor    string
}

// ScreeningRequest records one AML screening review decision input.
type ScreeningRequest struct {
	TenantID       valueobject.TenantID
	SubjectID      string
	Decision       string
	Note           string
	Actor          string
	IdempotencyKey string
}

// RegulatoryExportRequest renders one regulatory report type from E04-T05
// field definitions to a portable file.
type RegulatoryExportRequest struct {
	TenantID       valueobject.TenantID
	ReportType     string
	PeriodID       valueobject.PeriodID
	Fields         map[string]string
	Actor          string
	IdempotencyKey string
}

// ComplianceCommandUseCases defines the mutating operations on reconciliation, periods, reports, and screening. Strong writes.
type ComplianceCommandUseCases interface {
	// TriggerReconRun starts one reconciliation run. Strong write.
	TriggerReconRun(ctx context.Context, req ReconRunRequest) (ReconRunResult, error)
	// ResolveBreak resolves one break with SoD passthrough. Strong write.
	ResolveBreak(ctx context.Context, req BreakResolutionRequest) error
	// AcknowledgeBreak marks one break reviewed without adjustment. Strong write.
	AcknowledgeBreak(ctx context.Context, req BreakResolutionRequest) error
	// OpenPeriod opens one accounting period. Strong write.
	OpenPeriod(ctx context.Context, req PeriodRequest) error
	// ClosePeriod validates and closes one period, returning all failures. Strong write.
	ClosePeriod(ctx context.Context, req PeriodRequest) error
	// ReopenPeriod reopens a closed period with approval. Strong write.
	ReopenPeriod(ctx context.Context, req PeriodRequest) error
	// GenerateReport renders one of the 10 named report types. Strong write.
	GenerateReport(ctx context.Context, req ReportRequest) (ReportResult, error)
	// RecordScreeningDecision records one AML review decision. Strong write.
	RecordScreeningDecision(ctx context.Context, req ScreeningRequest) error
	// ExportRegulatoryReport renders one regulatory report type. Strong write.
	ExportRegulatoryReport(ctx context.Context, req RegulatoryExportRequest) (ReportResult, error)
}

// ComplianceQueryUseCases defines the read operations on compliance and reports. Strong reads.
type ComplianceQueryUseCases interface {
	// ReportStatus returns one report's generation state. Strong read.
	ReportStatus(ctx context.Context, tenant valueobject.TenantID, reportID string) (ReportResult, error)
}

// ComplianceUseCases is the composite inbound reconciliation/period/report/compliance
// surface (implemented in E06-T05). Period close and break resolution carry
// their SoD and completeness rules from the domain.
type ComplianceUseCases interface {
	ComplianceCommandUseCases
	ComplianceQueryUseCases
}
