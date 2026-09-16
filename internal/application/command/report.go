package command

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Report webhook event name (domain-events §3.11).
const EventReportGenerated = "report.generated.v1"

// ReportTemplates is the 10 named report types (features §6): six
// financial, four operational. Generation is parameterized over this list.
var ReportTemplates = []string{
	"trial-balance",
	"balance-sheet",
	"income-statement",
	"cash-flow-statement",
	"general-ledger-detail",
	"account-statements",
	"transaction-volume",
	"settlement",
	"fee-revenue",
	"exception-reports",
}

// Report lifecycle states.
const (
	ReportPending = "PENDING"
	ReportReady   = "READY"
)

// ReportRecord is the durable generation request with its delivery.
type ReportRecord struct {
	ID        string
	TenantID  valueobject.TenantID
	Template  string
	Status    string
	URL       string
	CreatedAt time.Time
}

// ReportStore is the consumer-owned report persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements.
type ReportStore interface {
	// CreateReport persists one PENDING generation. Strong write; fails on duplicate ID.
	CreateReport(ctx context.Context, record ReportRecord) error
	// FindReport returns one report by tenant + ID. Strong read.
	FindReport(ctx context.Context, tenant valueobject.TenantID, id string) (ReportRecord, error)
	// UpdateReport replaces one report record. Strong write.
	UpdateReport(ctx context.Context, record ReportRecord) error
	// ListReports returns one tenant's reports, newest first (bounded by the adapter). Strong read.
	ListReports(ctx context.Context, tenant valueobject.TenantID, limit int) ([]ReportRecord, error)
}

// ReportServiceParams encapsulates dependencies for ReportService.
type ReportServiceParams struct {
	UoW     port.UnitOfWork
	Reports ReportStore
	Storage port.ObjectStorage
	Clock   port.Clock
	IDs     port.IDGenerator
	Authz   port.Authorizer
}

// ReportService generates templated reports with signed-URL delivery.
// Summaries render from request parameters; projection-powered renderers
// arrive with later adapter work (documented, not silently partial: the
// payload carries template + parameters + generation time).
type ReportService struct {
	uow     port.UnitOfWork
	reports ReportStore
	storage port.ObjectStorage
	clock   port.Clock
	ids     port.IDGenerator
	authz   port.Authorizer
}

// NewReportService creates an encapsulated ReportService with validated dependencies.
func NewReportService(params ReportServiceParams) *ReportService {
	return &ReportService{
		uow:     params.UoW,
		reports: params.Reports,
		storage: params.Storage,
		clock:   params.Clock,
		ids:     params.IDs,
		authz:   params.Authz,
	}
}

// GenerateReport renders one named report type, stores its bytes, and
// delivers a signed URL with a generated fact. Strong write.
func (s *ReportService) GenerateReport(ctx context.Context, req port.ReportRequest) (port.ReportResult, error) {
	if err := validateReportEnvelope(req); err != nil {
		return port.ReportResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "report.generate", "tenant/"+string(req.TenantID)); err != nil {
		return port.ReportResult{}, err
	}
	now := s.clock.Now().UTC()
	body, err := renderReportSummary(req, now)
	if err != nil {
		return port.ReportResult{}, err
	}
	reportID := s.ids.NewID()
	key := "reports/" + string(req.TenantID) + "/" + reportID + ".json"
	if err := s.storage.Put(ctx, req.TenantID, key, port.StoredObject{Content: body, ContentType: "application/json"}); err != nil {
		return port.ReportResult{}, err
	}
	signed, err := s.storage.SignedURL(ctx, req.TenantID, key, time.Hour)
	if err != nil {
		return port.ReportResult{}, err
	}
	parts := append([]string{req.IdempotencyKey, string(req.TenantID), req.Template, req.Destination},
		MapParts("params", req.Parameters)...)
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(parts...),
		TenantID:    req.TenantID,
	}
	return stageReport(ctx, s.uow, rec, req.TenantID, reportID, req.Template, now,
		func(ctx context.Context) error {
			return s.reports.CreateReport(ctx, ReportRecord{
				ID: reportID, TenantID: req.TenantID, Template: req.Template,
				Status: ReportReady, URL: signed, CreatedAt: now,
			})
		},
		port.ReportResult{ReportID: reportID, Status: ReportReady, SignedURL: signed})
}

// stageReport reserves idempotency and persists one generated report with
// its generated fact in a single UnitOfWork. Storage bytes land before Do
// (same key, same bytes: overwrite-safe), so callbacks stay retry-safe.
func stageReport(ctx context.Context, uow port.UnitOfWork, rec port.IdempotencyRecord, tenant valueobject.TenantID, reportID, template string, now time.Time, persist func(ctx context.Context) error, result port.ReportResult) (port.ReportResult, error) {
	var out port.ReportResult
	err := uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeReportResult(outcome.Response)
			if err != nil {
				return err
			}
			out = decoded
			return nil
		}
		if err := persist(ctx); err != nil {
			return err
		}
		out = result
		out.Cursor = tx.Cursor()
		encoded, err := jsonparser.Marshal(out)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: tenant, EventType: EventReportGenerated,
			AggregateID: reportID, Payload: reportPayload(reportID, template), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.ReportResult{}, err
	}
	return out, nil
}

// ReportStatus returns one report's generation state. Strong read.
func (s *ReportService) ReportStatus(ctx context.Context, tenant valueobject.TenantID, reportID string) (port.ReportResult, error) {
	record, err := s.reports.FindReport(ctx, tenant, reportID)
	if err != nil {
		return port.ReportResult{}, err
	}
	return port.ReportResult{ReportID: record.ID, Status: record.Status, SignedURL: record.URL}, nil
}

// ListTemplates returns the supported report templates.
func (s *ReportService) ListTemplates() []string {
	return append([]string(nil), ReportTemplates...)
}

// validateReportEnvelope checks the generation envelope including template membership.
func validateReportEnvelope(req port.ReportRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if !isReportTemplate(req.Template) {
		return entity.NewError("REPORT_TEMPLATE_UNKNOWN", "report template is unknown")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "report actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "report requires an idempotency key")
	}
	return nil
}

// isReportTemplate reports membership in the 10 named types.
func isReportTemplate(template string) bool {
	for _, known := range ReportTemplates {
		if known == template {
			return true
		}
	}
	return false
}

// renderReportSummary renders the deterministic summary payload for one
// generation: template, parameters, and generation time.
func renderReportSummary(req port.ReportRequest, now time.Time) ([]byte, error) {
	return jsonparser.Marshal(map[string]any{
		"template":     req.Template,
		"parameters":   req.Parameters,
		"generated_at": now.UTC().Format(time.RFC3339),
	})
}

// reportPayload builds the §3.11-shaped fact payload: id plus template.
func reportPayload(id, template string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, "template": template})
	return encoded
}

// decodeReportResult restores a replayed response; corrupt records fail loudly.
func decodeReportResult(response []byte) (port.ReportResult, error) {
	var result port.ReportResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.ReportResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.ReportID == "" {
		return port.ReportResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
