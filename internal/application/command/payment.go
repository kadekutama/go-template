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

// processorTimeout bounds one provider call; ctx deadlines may tighten it.
const processorTimeout = 30 * time.Second

// Webhook payload keys shared by the §10 fact builders.
const (
	payloadIDKey       = "id"
	payloadStatusKey   = "status"
	payloadErrorKey    = "error"
	payloadOriginalKey = "original"
)

// Intent webhook event names (api-contracts §10). Handlers stage these facts;
// delivery and signing belong to E08.
const (
	EventIntentCreated        = "payment_intent.created.v1"
	EventIntentSucceeded      = "payment_intent.succeeded.v1"
	EventIntentFailed         = "payment_intent.failed.v1"
	EventIntentCanceled       = "payment_intent.canceled.v1"
	EventIntentRequiresAction = "payment_intent.requires_action.v1"
)

// Intent lifecycle states.
const (
	IntentPending        = "PENDING"
	IntentRequiresAction = "REQUIRES_ACTION"
	IntentConfirmed      = "CONFIRMED"
	IntentCanceled       = "CANCELED"
	IntentFailed         = "FAILED"
)

// IntentRecord is the durable payment-intent workflow state with its
// authorization accounting (E03-T08 capture rules apply at confirm).
type IntentRecord struct {
	ID            string
	TenantID      valueobject.TenantID
	LedgerID      valueobject.LedgerID
	AmountMinor   int64
	AssetCode     valueobject.AssetCode
	Method        valueobject.PaymentMethod
	Status        string
	ProviderID    string
	AuthID        string
	CapturedMinor int64
	ErrorCode     string
	CreatedAt     time.Time
}

// IntentStore is the consumer-owned intent persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements.
type IntentStore interface {
	// CreateIntent persists one PENDING intent. Strong write; fails on duplicate ID.
	CreateIntent(ctx context.Context, record IntentRecord) error
	// FindIntent returns one intent by tenant + ID. Strong read.
	FindIntent(ctx context.Context, tenant valueobject.TenantID, id string) (IntentRecord, error)
	// UpdateIntent replaces one intent record. Strong write.
	UpdateIntent(ctx context.Context, record IntentRecord) error
	// ListIntents returns one tenant's intents, newest first (bounded by the adapter). Strong read.
	ListIntents(ctx context.Context, tenant valueobject.TenantID, limit int) ([]IntentRecord, error)
}

// IntentService backs payment-intent charging over the payment-processor
// port. Provider calls happen OUTSIDE UnitOfWork callbacks (callbacks stay
// retry-safe); durable state transitions commit with outbox facts inside.
// IntentServiceParams carries dependencies for IntentService.
type IntentServiceParams struct {
	UoW       port.UnitOfWork
	Intents   IntentStore
	Processor port.PaymentProcessor
	Clock     port.Clock
	IDs       port.IDGenerator
	Authz     port.Authorizer
}

// IntentService backs payment-intent charging over the payment-processor
// port. Provider calls happen OUTSIDE UnitOfWork callbacks (callbacks stay
// retry-safe); durable state transitions commit with outbox facts inside.
type IntentService struct {
	uow       port.UnitOfWork
	intents   IntentStore
	processor port.PaymentProcessor
	clock     port.Clock
	ids       port.IDGenerator
	authz     port.Authorizer
}

// NewIntentService constructs an IntentService with the supplied dependencies.
func NewIntentService(params IntentServiceParams) *IntentService {
	return &IntentService{
		uow:       params.UoW,
		intents:   params.Intents,
		processor: params.Processor,
		clock:     params.Clock,
		ids:       params.IDs,
		authz:     params.Authz,
	}
}

// CreateIntent creates a PENDING intent with an authorized amount. Strong write.
func (s *IntentService) CreateIntent(ctx context.Context, req port.PaymentIntentRequest) (port.PaymentIntentResult, error) {
	if err := validateIntentEnvelope(req); err != nil {
		return port.PaymentIntentResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "payment.create", "ledger/"+string(req.LedgerID)); err != nil {
		return port.PaymentIntentResult{}, err
	}
	if _, err := service.Authorize("intent", req.AmountMinor, "intent", s.clock.Now().UTC(), service.DefaultAuthExpiryDays, false); err != nil {
		return port.PaymentIntentResult{}, err
	}
	intentID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), string(req.LedgerID), int64ToString(req.AmountMinor), string(req.AssetCode), string(req.Method)),
		TenantID:    req.TenantID,
	}
	var result port.PaymentIntentResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeIntentResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if err := s.intents.CreateIntent(ctx, IntentRecord{
			ID: intentID, TenantID: req.TenantID, LedgerID: req.LedgerID,
			AmountMinor: req.AmountMinor, AssetCode: req.AssetCode, Method: req.Method,
			Status: IntentPending, AuthID: "auth:" + intentID, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = port.PaymentIntentResult{IntentID: intentID, Status: IntentPending, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventIntentCreated,
			AggregateID: intentID, Payload: intentPayload(intentID, IntentPending, ""), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	return result, nil
}

// ConfirmIntent charges through the processor, then captures per domain
// rules. Processor failure leaves the intent PENDING for keyed retry; SCA
// challenges park it in REQUIRES_ACTION. Strong write.
func (s *IntentService) ConfirmIntent(ctx context.Context, req port.ConfirmIntentRequest) (port.PaymentIntentResult, error) {
	if err := validateConfirmEnvelope(req); err != nil {
		return port.PaymentIntentResult{}, err
	}
	subject := port.Subject{ID: req.Actor}
	if err := RequireAuthz(ctx, s.authz, subject, "payment.confirm", "intent/"+req.IntentID); err != nil {
		return port.PaymentIntentResult{}, err
	}
	intent, err := s.intents.FindIntent(ctx, req.TenantID, req.IntentID)
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	if intent.Status != IntentPending {
		// Terminal intents replay their stored outcome when the same key
		// resubmits; a fresh key on settled work is a state error.
		if result, replayed, err := s.checkIntentReplay(ctx, intentOpRecord(intent.TenantID, intent.ID, "confirm", req.IdempotencyKey, int64ToString(req.CaptureMinor))); err != nil || replayed {
			return result, err
		}
		return port.PaymentIntentResult{}, entity.NewError("INTENT_STATE_INVALID", "only pending intents can be confirmed")
	}
	charged, err := s.processor.Charge(ctx, port.ChargeRequest{
		TenantID: intent.TenantID, IntentID: intent.ID, AmountMinor: req.CaptureMinor, AssetCode: intent.AssetCode,
		Method: intent.Method, IdempotencyKey: req.IdempotencyKey, Timeout: processorTimeout,
	})
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	now := s.clock.Now().UTC()
	if charged.RequiresAction {
		return s.transitionIntent(ctx, intent, "confirm", req.IdempotencyKey, []string{int64ToString(req.CaptureMinor)}, IntentRequiresAction, charged.ProviderID, EventIntentRequiresAction, "", now)
	}
	if _, err := service.Capture(service.Authorization{
		ID: intent.AuthID, AmountMinor: intent.AmountMinor, CapturedMinor: intent.CapturedMinor,
		Status: valueobject.AuthAuthorized,
	}, req.CaptureMinor); err != nil {
		return port.PaymentIntentResult{}, err
	}
	return s.transitionIntent(ctx, withCapture(intent, req.CaptureMinor, charged.ProviderID), "confirm", req.IdempotencyKey, []string{int64ToString(req.CaptureMinor)}, IntentConfirmed, charged.ProviderID, EventIntentSucceeded, "", now)
}

// CancelIntent cancels an uncaptured intent. Strong write.
func (s *IntentService) CancelIntent(ctx context.Context, query port.PaymentQuery) (port.PaymentIntentResult, error) {
	intent, err := s.intents.FindIntent(ctx, valueobject.TenantID(query.TenantID), query.ID)
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	if strings.TrimSpace(query.Actor) == "" {
		return port.PaymentIntentResult{}, entity.NewError("ACTOR_REQUIRED", "payment actor is required")
	}
	subject := port.Subject{ID: query.Actor, TenantID: intent.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "payment.cancel", "intent/"+intent.ID); err != nil {
		return port.PaymentIntentResult{}, err
	}
	if result, replayed, err := s.checkIntentReplay(ctx, intentOpRecord(intent.TenantID, intent.ID, "cancel", "cancel:"+intent.ID)); err != nil || replayed {
		return result, err
	}
	if intent.Status != IntentPending && intent.Status != IntentRequiresAction {
		return port.PaymentIntentResult{}, entity.NewError("INTENT_STATE_INVALID", "only uncaptured intents can be canceled")
	}
	return s.transitionIntent(ctx, intent, "cancel", "cancel:"+intent.ID, nil, IntentCanceled, intent.ProviderID, EventIntentCanceled, "", s.clock.Now().UTC())
}

// CompleteChallenge resolves an SCA challenge: success captures in full,
// failure parks the intent FAILED with a failed fact. Strong write.
func (s *IntentService) CompleteChallenge(ctx context.Context, tenant valueobject.TenantID, intentID string, succeeded bool, actor, key string) (port.PaymentIntentResult, error) {
	intent, err := s.intents.FindIntent(ctx, tenant, intentID)
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "payment.confirm", "intent/"+intentID); err != nil {
		return port.PaymentIntentResult{}, err
	}
	if result, replayed, err := s.checkIntentReplay(ctx, intentOpRecord(tenant, intentID, "challenge", key, formatChallengeOutcome(succeeded))); err != nil || replayed {
		return result, err
	}
	if intent.Status != IntentRequiresAction {
		return port.PaymentIntentResult{}, entity.NewError("INTENT_STATE_INVALID", "only challenged intents can complete")
	}
	completed, err := s.processor.CompleteChallenge(ctx, port.ChallengeCompletion{
		TenantID: tenant, ProviderID: intent.ProviderID, Succeeded: succeeded, Timeout: processorTimeout,
	})
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	now := s.clock.Now().UTC()
	if !succeeded {
		return s.transitionIntent(ctx, intent, "challenge", key, []string{"false"}, IntentFailed, intent.ProviderID, EventIntentFailed, "CHALLENGE_FAILED", now)
	}
	if _, err := service.Capture(service.Authorization{
		ID: intent.AuthID, AmountMinor: intent.AmountMinor, CapturedMinor: intent.CapturedMinor,
		Status: valueobject.AuthAuthorized,
	}, intent.AmountMinor); err != nil {
		return port.PaymentIntentResult{}, err
	}
	_ = completed
	return s.transitionIntent(ctx, withCapture(intent, intent.AmountMinor, intent.ProviderID), "challenge", key, []string{"true"}, IntentConfirmed, intent.ProviderID, EventIntentSucceeded, "", now)
}

// GetIntent returns one payment intent. Strong read.
func (s *IntentService) GetIntent(ctx context.Context, query port.PaymentQuery) (port.PaymentIntentResult, error) {
	intent, err := s.intents.FindIntent(ctx, valueobject.TenantID(query.TenantID), query.ID)
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	return port.PaymentIntentResult{IntentID: intent.ID, Status: intent.Status}, nil
}

// ListIntents returns one tenant's intents, newest first (bounded). Strong read.
func (s *IntentService) ListIntents(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PaymentIntentResult, error) {
	records, err := s.intents.ListIntents(ctx, tenant, clampPageLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]port.PaymentIntentResult, 0, len(records))
	for _, record := range records {
		out = append(out, port.PaymentIntentResult{IntentID: record.ID, Status: record.Status})
	}
	return out, nil
}

// transitionIntent persists one intent state change with its webhook fact
// inside a single UnitOfWork keyed for idempotent retry. The fingerprint
// tags the operation: the same key across different operations conflicts
// instead of replaying the wrong outcome.
func (s *IntentService) transitionIntent(ctx context.Context, intent IntentRecord, op, key string, extra []string, status, providerID, eventType, errorCode string, now time.Time) (port.PaymentIntentResult, error) {
	rec := intentOpRecord(intent.TenantID, intent.ID, op, key, extra...)
	var result port.PaymentIntentResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeIntentResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		// Reload and compare status: a concurrent transition (cancel, second
		// confirm with another key) between load and commit must fail
		// loudly instead of overwriting. Same-key retries resolve through
		// replay above. (Captured totals are monotonic within one status
		// and re-checked by domain capture rules on the next attempt.)
		current, err := s.intents.FindIntent(ctx, intent.TenantID, intent.ID)
		if err != nil {
			return err
		}
		if current.Status != intent.Status {
			return entity.NewError("INTENT_CONCURRENT_MODIFICATION", "intent changed during processing; resubmit with the same key")
		}
		intent.Status = status
		intent.ProviderID = providerID
		intent.ErrorCode = errorCode
		if err := s.intents.UpdateIntent(ctx, intent); err != nil {
			return err
		}
		result = port.PaymentIntentResult{IntentID: intent.ID, Status: status, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: intent.TenantID, LedgerID: intent.LedgerID, EventType: eventType,
			AggregateID: intent.ID, Payload: intentPayload(intent.ID, status, errorCode), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.PaymentIntentResult{}, err
	}
	return result, nil
}

// formatChallengeOutcome canonicalizes the challenge decision for idempotency.
func formatChallengeOutcome(succeeded bool) string {
	if succeeded {
		return "true"
	}
	return "false"
}

// intentOpRecord builds the idempotency record for one intent operation.
// Reserving an uncompleted key with the identical fingerprint proceeds
// (reentrant for the same holder); a different fingerprint conflicts.
func intentOpRecord(tenant valueobject.TenantID, intentID, op, key string, extra ...string) port.IdempotencyRecord {
	parts := append([]string{key, intentID, op}, extra...)
	return port.IdempotencyRecord{
		Key:         key,
		Fingerprint: Fingerprint(parts...),
		TenantID:    tenant,
	}
}

// checkIntentReplay returns the stored outcome when key already completed
// the operation, reports whether it replayed, and leases the key otherwise.
func (s *IntentService) checkIntentReplay(ctx context.Context, rec port.IdempotencyRecord) (port.PaymentIntentResult, bool, error) {
	var result port.PaymentIntentResult
	replayed := false
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if !outcome.Replay {
			return nil
		}
		decoded, err := decodeIntentResult(outcome.Response)
		if err != nil {
			return err
		}
		result = decoded
		replayed = true
		return nil
	})
	if err != nil {
		return port.PaymentIntentResult{}, false, err
	}
	return result, replayed, nil
}

// withCapture records a successful capture against the intent.
func withCapture(intent IntentRecord, captured int64, providerID string) IntentRecord {
	intent.CapturedMinor += captured
	intent.ProviderID = providerID
	return intent
}

// validateConfirmEnvelope checks the confirm command envelope.
func validateConfirmEnvelope(req port.ConfirmIntentRequest) error {
	if strings.TrimSpace(req.IntentID) == "" {
		return entity.NewError("INTENT_ID_REQUIRED", "intent id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "payment actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payment requires an idempotency key")
	}
	if req.CaptureMinor <= 0 {
		return entity.NewError("INVALID_CAPTURE_AMOUNT", "capture amount must be positive")
	}
	return nil
}

// validateIntentEnvelope checks the intent creation envelope.
func validateIntentEnvelope(req port.PaymentIntentRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_INTENT_AMOUNT", "intent amount must be positive")
	}
	if strings.TrimSpace(string(req.AssetCode)) == "" {
		return entity.NewError("INTENT_ASSET_REQUIRED", "intent requires an asset code")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "payment actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payment requires an idempotency key")
	}
	return nil
}

// intentPayload builds the §10-shaped webhook payload: id plus status.
func intentPayload(id, status, errorCode string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadStatusKey: status, payloadErrorKey: errorCode})
	return encoded
}

// decodeIntentResult restores a replayed response; corrupt records fail loudly.
func decodeIntentResult(response []byte) (port.PaymentIntentResult, error) {
	var result port.PaymentIntentResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.PaymentIntentResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.IntentID == "" {
		return port.PaymentIntentResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
