package command

import (
	"context"
	"encoding/csv"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ScreeningDecision is the durable review record for one screened subject.
type ScreeningDecision struct {
	ID        string
	TenantID  valueobject.TenantID
	SubjectID string
	Decision  string
	Note      string
	Actor     string
	CreatedAt time.Time
}

// RegulatoryExport is the durable rendered regulatory file with delivery.
type RegulatoryExport struct {
	ID         string
	TenantID   valueobject.TenantID
	ReportType string
	URL        string
	CreatedAt  time.Time
}

// ComplianceStore is the consumer-owned compliance persistence boundary. The
// schema lands in E07-T10; this interface is the contract it implements.
type ComplianceStore interface {
	// RecordDecision persists one screening decision. Strong write; fails on duplicate ID.
	RecordDecision(ctx context.Context, decision ScreeningDecision) error
	// SaveExport persists one rendered regulatory file. Strong write; fails on duplicate ID.
	SaveExport(ctx context.Context, export RegulatoryExport) error
	// FindExport returns one export by tenant + ID. Strong read.
	FindExport(ctx context.Context, tenant valueobject.TenantID, id string) (RegulatoryExport, error)
}

// ComplianceServiceParams encapsulates dependencies for ComplianceService.
type ComplianceServiceParams struct {
	UoW     port.UnitOfWork
	Reviews ComplianceStore
	Storage port.ObjectStorage
	Clock   port.Clock
	IDs     port.IDGenerator
	Authz   port.Authorizer
}

// ComplianceService records screening reviews and renders regulatory exports
// from the E04-T05 field definitions. Transaction-time screening calls the
// AML port pre-posting (E09/E10 path); this service owns review auditability.
type ComplianceService struct {
	uow     port.UnitOfWork
	reviews ComplianceStore
	storage port.ObjectStorage
	clock   port.Clock
	ids     port.IDGenerator
	authz   port.Authorizer
}

// NewComplianceService creates an encapsulated ComplianceService with validated dependencies.
func NewComplianceService(params ComplianceServiceParams) *ComplianceService {
	return &ComplianceService{
		uow:     params.UoW,
		reviews: params.Reviews,
		storage: params.Storage,
		clock:   params.Clock,
		ids:     params.IDs,
		authz:   params.Authz,
	}
}

// RecordScreeningDecision records one AML review decision auditably. Strong write.
func (s *ComplianceService) RecordScreeningDecision(ctx context.Context, req port.ScreeningRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.SubjectID) == "" {
		return entity.NewError("SCREENING_SUBJECT_REQUIRED", "screening subject is required")
	}
	if strings.TrimSpace(req.Decision) == "" {
		return entity.NewError("SCREENING_DECISION_REQUIRED", "screening decision is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "compliance actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "compliance requires an idempotency key")
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "compliance.review", "tenant/"+string(req.TenantID)); err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), req.SubjectID, req.Decision),
		TenantID:    req.TenantID,
	}
	return s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			return nil
		}
		if err := s.reviews.RecordDecision(ctx, ScreeningDecision{
			ID: s.ids.NewID(), TenantID: req.TenantID, SubjectID: req.SubjectID,
			Decision: req.Decision, Note: req.Note, Actor: req.Actor, CreatedAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"subject":"`+req.SubjectID+`"}`))
	})
}

// ExportRegulatoryReport renders one regulatory type from its field
// definitions to CSV, stores it, and delivers a signed URL with a generated
// fact. Strong write.
func (s *ComplianceService) ExportRegulatoryReport(ctx context.Context, req port.RegulatoryExportRequest) (port.ReportResult, error) {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return port.ReportResult{}, entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return port.ReportResult{}, entity.NewError("ACTOR_REQUIRED", "compliance actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return port.ReportResult{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "compliance requires an idempotency key")
	}
	reportType := service.ReportType(req.ReportType)
	fields, err := service.ReportFields(reportType)
	if err != nil {
		return port.ReportResult{}, err
	}
	if err := service.ValidateReport(reportType, req.Fields); err != nil {
		return port.ReportResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "compliance.export", "tenant/"+string(req.TenantID)); err != nil {
		return port.ReportResult{}, err
	}
	body, err := renderRegulatoryCSV(fields, req.Fields)
	if err != nil {
		return port.ReportResult{}, err
	}
	exportID := s.ids.NewID()
	now := s.clock.Now().UTC()
	key := "regulatory/" + string(req.TenantID) + "/" + exportID + ".csv"
	if err := s.storage.Put(ctx, req.TenantID, key, port.StoredObject{Content: body, ContentType: "text/csv"}); err != nil {
		return port.ReportResult{}, err
	}
	signed, err := s.storage.SignedURL(ctx, req.TenantID, key, time.Hour)
	if err != nil {
		return port.ReportResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key: req.IdempotencyKey,
		Fingerprint: Fingerprint(append([]string{req.IdempotencyKey, string(req.TenantID), req.ReportType},
			MapParts("fields", req.Fields)...)...),
		TenantID: req.TenantID,
	}
	return stageReport(ctx, s.uow, rec, req.TenantID, exportID, req.ReportType, now,
		func(ctx context.Context) error {
			return s.reviews.SaveExport(ctx, RegulatoryExport{
				ID: exportID, TenantID: req.TenantID, ReportType: req.ReportType, URL: signed, CreatedAt: now,
			})
		},
		port.ReportResult{ReportID: exportID, Status: ReportReady, SignedURL: signed})
}

// renderRegulatoryCSV renders the definition-ordered header plus one values
// row. Column order always follows the field definition, never map iteration.
func renderRegulatoryCSV(header []string, values map[string]string) ([]byte, error) {
	row := make([]string, 0, len(header))
	for _, name := range header {
		row = append(row, values[name])
	}
	var buf strings.Builder
	writer := csv.NewWriter(&buf)
	if err := writer.Write(header); err != nil {
		return nil, err
	}
	if err := writer.Write(row); err != nil {
		return nil, err
	}
	writer.Flush()
	return []byte(buf.String()), writer.Error()
}
