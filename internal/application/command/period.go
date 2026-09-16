package command

import (
	"context"
	"errors"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Period webhook event names (domain-events §3.8).
const (
	EventPeriodOpened   = "period.opened.v1"
	EventPeriodClosed   = "period.closed.v1"
	EventPeriodReopened = "period.reopened.v1"
)

// PeriodStore is the consumer-owned period persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements.
type PeriodStore interface {
	// FindPeriod returns one period by tenant + ledger + ID. Strong read.
	FindPeriod(ctx context.Context, tenant, ledger, id string) (entity.PeriodData, error)
	// SavePeriod upserts one period record. Strong write.
	SavePeriod(ctx context.Context, period entity.PeriodData) error
	// ListPeriods returns one tenant/ledger's periods, newest first (bounded). Strong read.
	ListPeriods(ctx context.Context, tenant, ledger string, limit int) ([]entity.PeriodData, error)
}

// PeriodServiceParams carries dependencies for PeriodService.
type PeriodServiceParams struct {
	UoW     port.UnitOfWork
	Periods PeriodStore
	Breaks  ReconStore
	Clock   port.Clock
	IDs     port.IDGenerator
	Authz   port.Authorizer
}

// PeriodService opens, closes, and reopens accounting periods. Close runs
// the full validation gate and returns EVERY failure, never first-only.
type PeriodService struct {
	uow     port.UnitOfWork
	periods PeriodStore
	breaks  ReconStore
	clock   port.Clock
	ids     port.IDGenerator
	authz   port.Authorizer
}

// NewPeriodService constructs a PeriodService with the supplied dependencies.
func NewPeriodService(params PeriodServiceParams) *PeriodService {
	return &PeriodService{
		uow:     params.UoW,
		periods: params.Periods,
		breaks:  params.Breaks,
		clock:   params.Clock,
		ids:     params.IDs,
		authz:   params.Authz,
	}
}

// OpenPeriod opens one accounting period. Strong write.
func (s *PeriodService) OpenPeriod(ctx context.Context, req port.PeriodRequest) error {
	if err := validatePeriodOpen(req); err != nil {
		return err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "period.open", "ledger/"+string(req.LedgerID)); err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         periodKey(req.PeriodID, "open"),
		Fingerprint: Fingerprint(string(req.PeriodID), "open"),
		TenantID:    req.TenantID,
	}
	_ = now
	return s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			return nil
		}
		if err := s.periods.SavePeriod(ctx, entity.PeriodData{
			ID: req.PeriodID, TenantID: req.TenantID, LedgerID: req.LedgerID,
			Start: req.Start, End: req.End, Timezone: req.Timezone,
			Status: entity.PeriodOpen, Version: 1,
		}); err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventPeriodOpened,
			AggregateID: string(req.PeriodID), Payload: periodPayload(string(req.PeriodID), entity.PeriodOpen), OccurredAt: s.clock.Now().UTC(),
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"period_id":"`+string(req.PeriodID)+`"}`))
	})
}

// ClosePeriod validates every gate and closes, or returns all failures.
// Strong write.
func (s *PeriodService) ClosePeriod(ctx context.Context, req port.PeriodRequest) error {
	if err := validatePeriodScope(req); err != nil {
		return err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "period.close", "ledger/"+string(req.LedgerID)); err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         periodKey(req.PeriodID, "close"),
		Fingerprint: Fingerprint(string(req.PeriodID), "close"),
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
		period, err := s.periods.FindPeriod(ctx, string(req.TenantID), string(req.LedgerID), string(req.PeriodID))
		if err != nil {
			return err
		}
		openBreaks, err := s.breaks.CountOpenBreaks(ctx, req.TenantID, req.LedgerID)
		if err != nil {
			return err
		}
		if failures := service.ValidateClose(service.CloseInput{
			Period: period, UnresolvedWorkflowCount: req.UnresolvedWorkflows, OpenBreakCount: openBreaks,
			SubledgerDeltas: req.SubledgerDeltas, FXRevalued: req.FXRevalued,
		}); len(failures) > 0 {
			return errors.Join(failures...)
		}
		period.Status = entity.PeriodClosed
		if err := s.periods.SavePeriod(ctx, period); err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventPeriodClosed,
			AggregateID: string(req.PeriodID), Payload: periodPayload(string(req.PeriodID), entity.PeriodClosed), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"period_id":"`+string(req.PeriodID)+`"}`))
	})
}

// ReopenPeriod reopens a closed period with a second approver. Strong write.
func (s *PeriodService) ReopenPeriod(ctx context.Context, req port.PeriodRequest) error {
	if err := validatePeriodScope(req); err != nil {
		return err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "period.reopen", "ledger/"+string(req.LedgerID)); err != nil {
		return err
	}
	if err := service.ValidateReopen(req.Actor, req.Approver, "reopen "+string(req.PeriodID)); err != nil {
		return err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         periodKey(req.PeriodID, "reopen"),
		Fingerprint: Fingerprint(string(req.PeriodID), "reopen", req.Approver),
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
		period, err := s.periods.FindPeriod(ctx, string(req.TenantID), string(req.LedgerID), string(req.PeriodID))
		if err != nil {
			return err
		}
		if period.Status != entity.PeriodClosed {
			return entity.NewError("PERIOD_STATE_INVALID", "only closed periods can reopen")
		}
		period.Status = entity.PeriodOpen
		if err := s.periods.SavePeriod(ctx, period); err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventPeriodReopened,
			AggregateID: string(req.PeriodID), Payload: periodPayload(string(req.PeriodID), entity.PeriodOpen), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"period_id":"`+string(req.PeriodID)+`"}`))
	})
}

// validatePeriodOpen checks the open envelope including bounds.
func validatePeriodOpen(req port.PeriodRequest) error {
	if err := validatePeriodScope(req); err != nil {
		return err
	}
	if req.Start.IsZero() || req.End.IsZero() || !req.Start.Before(req.End) {
		return entity.NewError("PERIOD_BOUNDS_INVALID", "period start must precede end")
	}
	if strings.TrimSpace(req.Timezone) == "" {
		return entity.NewError("PERIOD_TIMEZONE_REQUIRED", "accounting timezone is required")
	}
	return nil
}

// validatePeriodScope checks the shared period envelope.
func validatePeriodScope(req port.PeriodRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(req.PeriodID.String()) == "" {
		return entity.NewError("PERIOD_ID_REQUIRED", "period id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "period actor is required")
	}
	return nil
}

// periodKey scopes period idempotency by period + action.
func periodKey(id valueobject.PeriodID, action string) string {
	return action + ":" + string(id)
}

// periodPayload builds the §3.8-shaped fact payload: id plus status.
func periodPayload(id, status string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadStatusKey: status})
	return encoded
}
