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

// Dispute webhook event names (api-contracts §10). Evidence and
// representment stage no webhook: the catalog carries opened/closed only.
const (
	EventDisputeOpened = "dispute.opened.v1"
	EventDisputeClosed = "dispute.closed.v1"
)

// DisputeStore is the consumer-owned dispute persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements.
type DisputeStore interface {
	// CreateDispute persists one OPEN dispute. Strong write; fails on duplicate ID.
	CreateDispute(ctx context.Context, dispute entity.Dispute) error
	// FindDispute returns one dispute by ID. Strong read.
	FindDispute(ctx context.Context, id string) (entity.Dispute, error)
	// UpdateDispute replaces one dispute record. Strong write.
	UpdateDispute(ctx context.Context, dispute entity.Dispute) error
	// ListDisputes pages disputes by status/date bounds. Point-in-time page.
	ListDisputes(ctx context.Context, status string, from, to time.Time, cursor string, limit int) ([]entity.Dispute, string, error)
}

// DisputeServiceParams carries dependencies for DisputeService.
type DisputeServiceParams struct {
	UoW               port.UnitOfWork
	Disputes          DisputeStore
	Policies          map[string]service.NetworkPolicy
	SoDThresholdMinor int64
	Clock             port.Clock
	IDs               port.IDGenerator
	Authz             port.Authorizer
}

// DisputeService faces the dispute domain to the edge: windows, evidence
// deadlines, versioned representment, and SoD-gated closure. The domain owns
// every rule; the handler books stages around them.
type DisputeService struct {
	uow               port.UnitOfWork
	disputes          DisputeStore
	policies          map[string]service.NetworkPolicy
	soDThresholdMinor int64
	clock             port.Clock
	ids               port.IDGenerator
	authz             port.Authorizer
}

var _ port.DisputeCommandUseCases = (*DisputeService)(nil)

// NewDisputeService constructs a DisputeService with the supplied dependencies.
func NewDisputeService(params DisputeServiceParams) *DisputeService {
	return &DisputeService{
		uow:               params.UoW,
		disputes:          params.Disputes,
		policies:          params.Policies,
		soDThresholdMinor: params.SoDThresholdMinor,
		clock:             params.Clock,
		ids:               params.IDs,
		authz:             params.Authz,
	}
}

// OpenDispute validates the network window and holds funds with a fee.
// Strong write.
func (s *DisputeService) OpenDispute(ctx context.Context, cmd port.OpenDisputeCommand) (port.DisputeResult, error) {
	if err := validateDisputeOpen(cmd); err != nil {
		return port.DisputeResult{}, err
	}
	subject := port.Subject{ID: cmd.Actor}
	if err := RequireAuthz(ctx, s.authz, subject, "dispute.open", "payment/"+cmd.PaymentID); err != nil {
		return port.DisputeResult{}, err
	}
	policy, err := s.policyFor(cmd.Network)
	if err != nil {
		return port.DisputeResult{}, err
	}
	disputeID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         cmd.IdempotencyKey,
		Fingerprint: Fingerprint(cmd.IdempotencyKey, cmd.PaymentID, cmd.OriginalPostingID, cmd.Network, int64ToString(cmd.AmountMinor)),
	}
	var result port.DisputeResult
	err = s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeDisputeResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		dispute, err := service.OpenDispute(service.OpenDisputeRequest{
			DisputeID: disputeID, PaymentID: cmd.PaymentID, OriginalPostingID: cmd.OriginalPostingID,
			Network: cmd.Network, AmountMinor: cmd.AmountMinor, PaymentAt: cmd.PaymentAt, Now: now,
			HoldID: "hold:" + disputeID, Policy: policy,
		})
		if err != nil {
			return err
		}
		if err := s.disputes.CreateDispute(ctx, dispute); err != nil {
			return err
		}
		result = port.DisputeResult{Dispute: dispute, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			EventType: EventDisputeOpened, AggregateID: disputeID,
			Payload: disputePayload(disputeID, string(dispute.Status), ""), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.DisputeResult{}, err
	}
	return result, nil
}

// SubmitEvidence files evidence before the deadline, auto-marking OPEN
// disputes EVIDENCE_DUE first (both steps are domain transitions). Strong write.
func (s *DisputeService) SubmitEvidence(ctx context.Context, cmd port.EvidenceCommand) (port.DisputeResult, error) {
	if err := validateDisputeKey(cmd.DisputeID, cmd.Actor, cmd.IdempotencyKey); err != nil {
		return port.DisputeResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         cmd.IdempotencyKey,
		Fingerprint: Fingerprint(cmd.IdempotencyKey, cmd.DisputeID, "evidence"),
	}
	now := s.clock.Now().UTC()
	return s.runDisputeCommand(ctx, port.Subject{ID: cmd.Actor}, "dispute.evidence", "dispute/"+cmd.DisputeID, rec,
		func(ctx context.Context, _ port.Tx) (entity.Dispute, *port.OutboxFact, error) {
			stored, err := s.disputes.FindDispute(ctx, cmd.DisputeID)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			if stored.Status == valueobject.DisputeOpen {
				stored, err = service.MarkEvidenceDue(stored)
				if err != nil {
					return entity.Dispute{}, nil, err
				}
			}
			updated, err := service.SubmitEvidence(stored, now)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			if err := s.disputes.UpdateDispute(ctx, updated); err != nil {
				return entity.Dispute{}, nil, err
			}
			return updated, nil, nil
		})
}

// RepresentDispute files one representment stage under the versioned network
// allowance. The domain gates the allowance; the stage count is workflow
// bookkeeping persisted with the record. Strong write.
func (s *DisputeService) RepresentDispute(ctx context.Context, cmd port.RepresentCommand) (port.DisputeResult, error) {
	if err := validateDisputeKey(cmd.DisputeID, cmd.Actor, cmd.IdempotencyKey); err != nil {
		return port.DisputeResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         cmd.IdempotencyKey,
		Fingerprint: Fingerprint(cmd.IdempotencyKey, cmd.DisputeID, "represent"),
	}
	return s.runDisputeCommand(ctx, port.Subject{ID: cmd.Actor}, "dispute.represent", "dispute/"+cmd.DisputeID, rec,
		func(ctx context.Context, _ port.Tx) (entity.Dispute, *port.OutboxFact, error) {
			stored, err := s.disputes.FindDispute(ctx, cmd.DisputeID)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			policy, err := s.policyFor(stored.Network)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			if err := service.RepresentmentAllowed(stored, policy); err != nil {
				return entity.Dispute{}, nil, err
			}
			stored.RepresentStage++
			if err := s.disputes.UpdateDispute(ctx, stored); err != nil {
				return entity.Dispute{}, nil, err
			}
			return stored, nil, nil
		})
}

// CloseDispute decides won/lost with SoD passthrough: above-threshold closes
// require an approver different from the actor (E04-T02 rule surfaced here,
// not re-implemented). Strong write.
func (s *DisputeService) CloseDispute(ctx context.Context, cmd port.CloseDisputeCommand) (port.DisputeResult, error) {
	if err := validateDisputeKey(cmd.DisputeID, cmd.Actor, cmd.IdempotencyKey); err != nil {
		return port.DisputeResult{}, err
	}
	decision, err := parseDisputeOutcome(cmd.Outcome)
	if err != nil {
		return port.DisputeResult{}, err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         cmd.IdempotencyKey,
		Fingerprint: Fingerprint(cmd.IdempotencyKey, cmd.DisputeID, cmd.Outcome, cmd.Approver),
	}
	return s.runDisputeCommand(ctx, port.Subject{ID: cmd.Actor}, "dispute.close", "dispute/"+cmd.DisputeID, rec,
		func(ctx context.Context, _ port.Tx) (entity.Dispute, *port.OutboxFact, error) {
			stored, err := s.disputes.FindDispute(ctx, cmd.DisputeID)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			if err := checkDisputeSoD(stored, cmd.Actor, cmd.Approver, s.soDThresholdMinor); err != nil {
				return entity.Dispute{}, nil, err
			}
			closed, err := service.CloseDispute(stored, decision)
			if err != nil {
				return entity.Dispute{}, nil, err
			}
			if err := s.disputes.UpdateDispute(ctx, closed); err != nil {
				return entity.Dispute{}, nil, err
			}
			return closed, &port.OutboxFact{
				EventType: EventDisputeClosed, AggregateID: closed.ID,
				Payload: disputePayload(closed.ID, string(closed.Status), string(decision)), OccurredAt: now,
			}, nil
		})
}

// runDisputeCommand authorizes, reserves idempotency inside one UnitOfWork,
// and either replays the stored response or runs the mutation exactly once,
// staging the optional webhook fact with the writes.
func (s *DisputeService) runDisputeCommand(ctx context.Context, subject port.Subject, action, resource string, rec port.IdempotencyRecord,
	mutate func(ctx context.Context, tx port.Tx) (entity.Dispute, *port.OutboxFact, error),
) (port.DisputeResult, error) {
	if err := RequireAuthz(ctx, s.authz, subject, action, resource); err != nil {
		return port.DisputeResult{}, err
	}
	var result port.DisputeResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeDisputeResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		dispute, fact, err := mutate(ctx, tx)
		if err != nil {
			return err
		}
		result = port.DisputeResult{Dispute: dispute, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if fact != nil {
			if err := tx.Outbox().Append(ctx, *fact); err != nil {
				return err
			}
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.DisputeResult{}, err
	}
	return result, nil
}

// validateDisputeKey checks the dispute command envelope shared by
// evidence, representment, and closure.
func validateDisputeKey(disputeID, actor, key string) error {
	if strings.TrimSpace(disputeID) == "" {
		return entity.NewError("DISPUTE_ID_REQUIRED", "dispute id is required")
	}
	if strings.TrimSpace(actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "dispute actor is required")
	}
	if strings.TrimSpace(key) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "dispute requires an idempotency key")
	}
	return nil
}

// parseDisputeOutcome validates the close outcome.
func parseDisputeOutcome(outcome string) (service.DisputeOutcome, error) {
	switch service.DisputeOutcome(outcome) {
	case service.DisputeWon:
		return service.DisputeWon, nil
	case service.DisputeLost:
		return service.DisputeLost, nil
	default:
		return "", entity.NewError("DISPUTE_OUTCOME_INVALID", "dispute outcome must be WON or LOST")
	}
}

// checkDisputeSoD surfaces the E04-T02 segregation rule: above-threshold
// closes require an approver different from the actor.
func checkDisputeSoD(dispute entity.Dispute, actor, approver string, thresholdMinor int64) error {
	if dispute.AmountMinor <= thresholdMinor {
		return nil
	}
	if strings.TrimSpace(approver) == "" || approver == actor {
		return entity.NewError("SOD_VIOLATION", "above-threshold close requires a different approver")
	}
	return nil
}

// policyFor resolves the versioned network policy or fails closed.
func (s *DisputeService) policyFor(network string) (service.NetworkPolicy, error) {
	policy, ok := s.policies[network]
	if !ok {
		return service.NetworkPolicy{}, entity.NewError("DISPUTE_POLICY_UNKNOWN", "no network policy for "+network)
	}
	return policy, nil
}

// validateDisputeOpen checks the open envelope.
func validateDisputeOpen(cmd port.OpenDisputeCommand) error {
	if strings.TrimSpace(cmd.PaymentID) == "" || strings.TrimSpace(cmd.OriginalPostingID) == "" {
		return entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting")
	}
	if strings.TrimSpace(cmd.Network) == "" {
		return entity.NewError("DISPUTE_NETWORK_REQUIRED", "dispute requires a network")
	}
	if cmd.AmountMinor <= 0 {
		return entity.NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive")
	}
	if strings.TrimSpace(cmd.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "dispute actor is required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "dispute requires an idempotency key")
	}
	return nil
}

// disputePayload builds the §10-shaped webhook payload: id, status, outcome.
func disputePayload(id, status, outcome string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadStatusKey: status, "outcome": outcome})
	return encoded
}

// decodeDisputeResult restores a replayed response; corrupt records fail loudly.
func decodeDisputeResult(response []byte) (port.DisputeResult, error) {
	var result port.DisputeResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.DisputeResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.Dispute.ID == "" {
		return port.DisputeResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
