package command

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Reconciliation webhook event names (domain-events §3.7).
const (
	EventReconRunStarted        = "reconciliation.run.started.v1"
	EventReconRunCompleted      = "reconciliation.run.completed.v1"
	EventReconBreakFound        = "reconciliation.break.found.v1"
	EventReconBreakResolved     = "reconciliation.break.resolved.v1"
	EventReconBreakAcknowledged = "reconciliation.break.acknowledged.v1"
)

// Recon run lifecycle states.
const (
	ReconRunPending   = "PENDING"
	ReconRunCompleted = "COMPLETED"
)

// ReconRunRecord is the durable reconciliation run.
type ReconRunRecord struct {
	ID          string
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	Source      string
	WindowStart time.Time
	WindowEnd   time.Time
	Status      string
	CreatedAt   time.Time
}

// BreakRecord carries a detected break with its workflow state. The domain
// break holds detection facts; status/decisions are workflow state owned
// here (the domain break has no status field).
type BreakRecord struct {
	Break     entity.ReconciliationBreak
	Status    valueobject.BreakStatus
	DecidedBy string
	DecidedAt time.Time
}

// ReconStore is the consumer-owned reconciliation persistence boundary. The
// schema lands in E07-T10; this interface is the contract it implements.
type ReconStore interface {
	// CreateRun persists one PENDING run. Strong write; fails on duplicate ID.
	CreateRun(ctx context.Context, run ReconRunRecord) error
	// FindRun returns one run by tenant + ID. Strong read.
	FindRun(ctx context.Context, tenant valueobject.TenantID, id string) (ReconRunRecord, error)
	// UpdateRun replaces one run record. Strong write.
	UpdateRun(ctx context.Context, run ReconRunRecord) error
	// CreateBreaks persists detected breaks. Strong write.
	CreateBreaks(ctx context.Context, breaks []BreakRecord) error
	// FindBreak returns one break by ID. Strong read.
	FindBreak(ctx context.Context, tenant valueobject.TenantID, id string) (BreakRecord, error)
	// ListBreaksByRun returns every break of one run. Strong read (added in E06-T09 for run summaries).
	ListBreaksByRun(ctx context.Context, tenant valueobject.TenantID, runID string) ([]BreakRecord, error)
	// UpdateBreak replaces one break record. Strong write.
	UpdateBreak(ctx context.Context, record BreakRecord) error
	// CountOpenBreaks counts unresolved breaks for period gating. Strong read.
	CountOpenBreaks(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID) (int, error)
	// ListRuns returns one tenant's runs, newest first (bounded by the adapter). Strong read.
	ListRuns(ctx context.Context, tenant valueobject.TenantID, limit int) ([]ReconRunRecord, error)
	// ListBreaks returns one tenant's breaks by status (empty = all), bounded. Strong read.
	ListBreaks(ctx context.Context, tenant valueobject.TenantID, status string, limit int) ([]BreakRecord, error)
}

// MatchInput carries the data + versioned rule inputs for one execution.
type MatchInput struct {
	Ledger   []service.LedgerFact
	External []entity.ExternalStatementLine
	Config   service.MatchConfig
}

// ReconServiceParams carries dependencies for ReconService.
type ReconServiceParams struct {
	UoW               port.UnitOfWork
	Runs              ReconStore
	Clock             port.Clock
	IDs               port.IDGenerator
	Authz             port.Authorizer
	SoDThresholdMinor int64
}

// ReconService triggers runs, executes matching, and resolves breaks with
// SoD passthrough. The domain owns matching, transitions, and SoD.
type ReconService struct {
	uow               port.UnitOfWork
	runs              ReconStore
	clock             port.Clock
	ids               port.IDGenerator
	authz             port.Authorizer
	soDThresholdMinor int64
}

// NewReconService constructs a ReconService with the supplied dependencies.
func NewReconService(params ReconServiceParams) *ReconService {
	return &ReconService{
		uow:               params.UoW,
		runs:              params.Runs,
		clock:             params.Clock,
		ids:               params.IDs,
		authz:             params.Authz,
		soDThresholdMinor: params.SoDThresholdMinor,
	}
}

// TriggerReconRun starts one PENDING run. Strong write.
func (s *ReconService) TriggerReconRun(ctx context.Context, req port.ReconRunRequest) (port.ReconRunResult, error) {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return port.ReconRunResult{}, entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return port.ReconRunResult{}, entity.NewError("ACTOR_REQUIRED", "reconciliation actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return port.ReconRunResult{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "reconciliation requires an idempotency key")
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "recon.trigger", "ledger/"+string(req.LedgerID)); err != nil {
		return port.ReconRunResult{}, err
	}
	runID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), req.Source, req.WindowStart.UTC().Format(time.RFC3339Nano), req.WindowEnd.UTC().Format(time.RFC3339Nano)),
		TenantID:    req.TenantID,
	}
	var result port.ReconRunResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeReconResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if err := s.runs.CreateRun(ctx, ReconRunRecord{
			ID: runID, TenantID: req.TenantID, LedgerID: req.LedgerID, Source: req.Source,
			WindowStart: req.WindowStart, WindowEnd: req.WindowEnd, Status: ReconRunPending, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = port.ReconRunResult{RunID: runID, Status: ReconRunPending, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventReconRunStarted,
			AggregateID: runID, Payload: reconPayload(runID, ReconRunPending), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.ReconRunResult{}, err
	}
	return result, nil
}

// ExecuteRun matches ledger facts against statement lines, persists breaks,
// and completes the run with per-break facts. Driven by workers (E14);
// strong write. The run loads inside, after matching inputs validate.
func (s *ReconService) ExecuteRun(ctx context.Context, tenant valueobject.TenantID, runID string, input MatchInput) (port.ReconRunResult, error) {
	now := s.clock.Now().UTC()
	var result port.ReconRunResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		run, err := s.runs.FindRun(ctx, tenant, runID)
		if err != nil {
			return err
		}
		if run.Status != ReconRunPending {
			return entity.NewError("RUN_STATE_INVALID", "only pending runs can execute")
		}
		_, breaks, err := service.MatchStatements(input.Ledger, input.External, input.Config)
		if err != nil {
			return err
		}
		records := make([]BreakRecord, 0, len(breaks))
		for _, br := range breaks {
			records = append(records, BreakRecord{Break: br, Status: valueobject.BreakOpen})
		}
		if err := s.runs.CreateBreaks(ctx, records); err != nil {
			return err
		}
		run.Status = ReconRunCompleted
		if err := s.runs.UpdateRun(ctx, run); err != nil {
			return err
		}
		result = port.ReconRunResult{RunID: runID, Status: ReconRunCompleted, Cursor: tx.Cursor()}
		facts := []port.OutboxFact{{
			TenantID: run.TenantID, LedgerID: run.LedgerID, EventType: EventReconRunCompleted,
			AggregateID: runID, Payload: reconPayload(runID, ReconRunCompleted), OccurredAt: now,
		}}
		for _, br := range breaks {
			facts = append(facts, port.OutboxFact{
				TenantID: run.TenantID, LedgerID: run.LedgerID, EventType: EventReconBreakFound,
				AggregateID: br.BreakID, Payload: reconPayload(br.BreakID, string(br.Type)), OccurredAt: now,
			})
		}
		return tx.Outbox().Append(ctx, facts...)
	})
	if err != nil {
		return port.ReconRunResult{}, err
	}
	return result, nil
}

// ResolveBreak resolves one break with SoD passthrough. Strong write.
func (s *ReconService) ResolveBreak(ctx context.Context, req port.BreakResolutionRequest) error {
	return s.decideBreak(ctx, req, false)
}

// AcknowledgeBreak marks one break reviewed without adjustment. Strong write.
func (s *ReconService) AcknowledgeBreak(ctx context.Context, req port.BreakResolutionRequest) error {
	return s.decideBreak(ctx, req, true)
}

// decideBreak validates the resolution through the domain (evidence,
// transition, amounts, SoD) and persists the decided state with its fact.
// Replay returns nil: the decision already holds.
func (s *ReconService) decideBreak(ctx context.Context, req port.BreakResolutionRequest, acknowledge bool) error {
	if err := validateBreakEnvelope(req); err != nil {
		return err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "recon.resolve", "recon/"+req.BreakID); err != nil {
		return err
	}
	action := valueobject.ResolveAdjustLedger
	target := valueobject.BreakResolved
	eventType := EventReconBreakResolved
	if acknowledge {
		action = valueobject.ResolveAcknowledge
		target = valueobject.BreakAcknowledged
		eventType = EventReconBreakAcknowledged
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, req.BreakID, string(action), req.Decision, req.Approver),
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
		stored, err := s.runs.FindBreak(ctx, req.TenantID, req.BreakID)
		if err != nil {
			return err
		}
		if err := service.ValidateResolution(service.ResolutionRequest{
			BreakID: req.BreakID, Action: action, ReasonCode: req.Decision, EvidenceRef: req.Note,
			Owner: req.Actor, Actor: req.Actor, Approver: req.Approver, AuditLink: "audit:" + req.BreakID,
			AdjustmentPostingID: req.AdjustmentPostingID,
			AmountMinor:         adjustmentAmount(stored.Break),
			ExpiresAt:           now.Add(30 * 24 * time.Hour), Now: now,
		}, stored.Status, s.soDThresholdMinor); err != nil {
			return err
		}
		stored.Status = target
		stored.DecidedBy = req.Actor
		stored.DecidedAt = now
		if err := s.runs.UpdateBreak(ctx, stored); err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, EventType: eventType,
			AggregateID: req.BreakID, Payload: reconPayload(req.BreakID, string(target)), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"break_id":"`+req.BreakID+`","status":"`+string(target)+`"}`))
	})
}

// adjustmentAmount derives the correction delta from a detected break: the
// absolute difference between expected and actual legs. Missing-side breaks
// book the present side; amount breaks book the delta.
func adjustmentAmount(br entity.ReconciliationBreak) int64 {
	if br.ExpectedMinor >= br.ActualMinor {
		return br.ExpectedMinor - br.ActualMinor
	}
	return br.ActualMinor - br.ExpectedMinor
}

// validateBreakEnvelope checks the resolution envelope.
func validateBreakEnvelope(req port.BreakResolutionRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.BreakID) == "" {
		return entity.NewError("BREAK_ID_REQUIRED", "break id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "reconciliation actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "reconciliation requires an idempotency key")
	}
	return nil
}

// reconPayload builds the §3.7-shaped webhook payload: id plus status.
func reconPayload(id, status string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadStatusKey: status})
	return encoded
}

// decodeReconResult restores a replayed run response; corrupt records fail loudly.
func decodeReconResult(response []byte) (port.ReconRunResult, error) {
	var result port.ReconRunResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.ReconRunResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.RunID == "" {
		return port.ReconRunResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
