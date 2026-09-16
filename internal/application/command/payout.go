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

// Payout webhook event names (api-contracts §10).
const (
	EventPayoutCreated = "payout.created.v1"
	EventPayoutPending = "payout.pending.v1"
	EventPayoutPaid    = "payout.paid.v1"
	EventPayoutFailed  = "payout.failed.v1"
)

// Payout lifecycle states (submission → settlement tracking).
const (
	PayoutPending  = "PENDING"
	PayoutPaid     = "PAID"
	PayoutFailed   = "FAILED"
	PayoutCanceled = "CANCELED"
)

// Topup lifecycle states.
const (
	TopupPending  = "PENDING"
	TopupCanceled = "CANCELED"
)

// PayoutRecord is the durable two-stage payout with its submission linkage.
type PayoutRecord struct {
	ID          string
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	AccountID   valueobject.AccountID
	AmountMinor int64
	AssetCode   valueobject.AssetCode
	Method      valueobject.PayoutMethod
	Status      string
	ErrorCode   string
	CreatedAt   time.Time
}

// PayoutStore is the consumer-owned payout persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements. Policies
// are versioned configuration supplied by the operator.
type PayoutStore interface {
	// CreatePayout persists one staged payout. Strong write; fails on duplicate ID.
	CreatePayout(ctx context.Context, record PayoutRecord) error
	// FindPayout returns one payout by tenant + ID. Strong read.
	FindPayout(ctx context.Context, tenant valueobject.TenantID, id string) (PayoutRecord, error)
	// UpdatePayout replaces one payout record. Strong write.
	UpdatePayout(ctx context.Context, record PayoutRecord) error
	// GetPolicy returns the tenant/asset payout policy. Strong read.
	GetPolicy(ctx context.Context, tenant valueobject.TenantID, asset valueobject.AssetCode) (valueobject.PayoutPolicy, error)
	// UpdatePolicy replaces the tenant/asset payout policy. Strong write.
	UpdatePolicy(ctx context.Context, policy valueobject.PayoutPolicy) error
	// ListPayouts returns one tenant's payouts, newest first (bounded by the adapter). Strong read.
	ListPayouts(ctx context.Context, tenant valueobject.TenantID, limit int) ([]PayoutRecord, error)
}

// TopupRecord is the durable top-up intent.
type TopupRecord struct {
	ID          string
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	AccountID   valueobject.AccountID
	AmountMinor int64
	AssetCode   valueobject.AssetCode
	Status      string
	CreatedAt   time.Time
}

// TopupStore is the consumer-owned top-up persistence boundary (E07-T10).
type TopupStore interface {
	// CreateTopup persists one PENDING top-up. Strong write; fails on duplicate ID.
	CreateTopup(ctx context.Context, record TopupRecord) error
	// FindTopup returns one top-up by tenant + ID. Strong read.
	FindTopup(ctx context.Context, tenant valueobject.TenantID, id string) (TopupRecord, error)
	// UpdateTopup replaces one top-up record. Strong write.
	UpdateTopup(ctx context.Context, record TopupRecord) error
	// ListTopups returns one tenant's top-ups, newest first (bounded by the adapter). Strong read.
	ListTopups(ctx context.Context, tenant valueobject.TenantID, limit int) ([]TopupRecord, error)
}

// PayoutServiceParams carries dependencies for PayoutService.
type PayoutServiceParams struct {
	UoW            port.UnitOfWork
	Payouts        PayoutStore
	Balances       port.GetBalance
	TransitAccount valueobject.AccountID
	Clock          port.Clock
	IDs            port.IDGenerator
	Authz          port.Authorizer
}

// PayoutService gates payouts on the eligibility policy, then stages the
// two-stage submission. Ineligible requests fail with PAYOUT_BLOCKED reason
// details and never reach any provider.
type PayoutService struct {
	uow            port.UnitOfWork
	payouts        PayoutStore
	balances       port.GetBalance
	transitAccount valueobject.AccountID
	clock          port.Clock
	ids            port.IDGenerator
	authz          port.Authorizer
}

// NewPayoutService constructs a PayoutService with the supplied dependencies.
func NewPayoutService(params PayoutServiceParams) *PayoutService {
	return &PayoutService{
		uow:            params.UoW,
		payouts:        params.Payouts,
		balances:       params.Balances,
		transitAccount: params.TransitAccount,
		clock:          params.Clock,
		ids:            params.IDs,
		authz:          params.Authz,
	}
}

// CreatePayout gates on eligibility, then stages submission. Strong write.
// Reads evaluate inside the UnitOfWork after the replay check (data-flow
// §2 order); replays skip them.
func (s *PayoutService) CreatePayout(ctx context.Context, req port.PayoutRequest) (port.PayoutResult, error) {
	if err := validatePayoutEnvelope(req); err != nil {
		return port.PayoutResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "payout.create", "ledger/"+string(req.LedgerID)); err != nil {
		return port.PayoutResult{}, err
	}
	payoutID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), string(req.AccountID), int64ToString(req.AmountMinor), string(req.AssetCode), string(req.Method)),
		TenantID:    req.TenantID,
	}
	var result port.PayoutResult
	doErr := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodePayoutResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if err := s.checkEligibility(ctx, req); err != nil {
			return err
		}
		if err := s.payouts.CreatePayout(ctx, PayoutRecord{
			ID: payoutID, TenantID: req.TenantID, LedgerID: req.LedgerID, AccountID: req.AccountID,
			AmountMinor: req.AmountMinor, AssetCode: req.AssetCode, Method: req.Method,
			Status: PayoutPending, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = port.PayoutResult{PayoutID: payoutID, Status: PayoutPending, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: EventPayoutCreated,
			AggregateID: payoutID, Payload: payoutPayload(payoutID, PayoutPending, ""), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if doErr != nil {
		return port.PayoutResult{}, doErr
	}
	return result, nil
}

// CancelPayout cancels a PENDING payout. Strong write.
func (s *PayoutService) CancelPayout(ctx context.Context, query port.PaymentQuery) (port.PayoutResult, error) {
	if strings.TrimSpace(query.Actor) == "" {
		return port.PayoutResult{}, entity.NewError("ACTOR_REQUIRED", "payout actor is required")
	}
	subject := port.Subject{ID: query.Actor, TenantID: valueobject.TenantID(query.TenantID)}
	if err := RequireAuthz(ctx, s.authz, subject, "payout.cancel", "payout/"+query.ID); err != nil {
		return port.PayoutResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         "cancel:" + query.ID,
		Fingerprint: Fingerprint("cancel", query.ID),
		TenantID:    valueobject.TenantID(query.TenantID),
	}
	var result port.PayoutResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodePayoutResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		record, err := s.payouts.FindPayout(ctx, valueobject.TenantID(query.TenantID), query.ID)
		if err != nil {
			return err
		}
		if record.Status != PayoutPending {
			return entity.NewError("PAYOUT_STATE_INVALID", "only pending payouts can be canceled")
		}
		record.Status = PayoutCanceled
		if err := s.payouts.UpdatePayout(ctx, record); err != nil {
			return err
		}
		result = port.PayoutResult{PayoutID: record.ID, Status: PayoutCanceled, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		// No webhook: the catalog carries created/pending/paid/failed; a
		// cancel is queryable state, like tenant updates and evidence.
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.PayoutResult{}, err
	}
	return result, nil
}

// GetPayout returns one payout. Strong read.
func (s *PayoutService) GetPayout(ctx context.Context, query port.PaymentQuery) (port.PayoutResult, error) {
	record, err := s.payouts.FindPayout(ctx, valueobject.TenantID(query.TenantID), query.ID)
	if err != nil {
		return port.PayoutResult{}, err
	}
	return port.PayoutResult{PayoutID: record.ID, Status: record.Status}, nil
}

// ListPayouts returns one tenant's payouts, newest first (bounded). Strong read.
func (s *PayoutService) ListPayouts(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PayoutResult, error) {
	records, err := s.payouts.ListPayouts(ctx, tenant, clampPageLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]port.PayoutResult, 0, len(records))
	for _, record := range records {
		out = append(out, port.PayoutResult{PayoutID: record.ID, Status: record.Status})
	}
	return out, nil
}

// GetSchedule returns the tenant/asset payout policy. Strong read.
func (s *PayoutService) GetSchedule(ctx context.Context, tenant valueobject.TenantID, asset valueobject.AssetCode) (valueobject.PayoutPolicy, error) {
	return s.payouts.GetPolicy(ctx, tenant, asset)
}

// UpdateSchedule replaces the tenant/asset payout policy. Strong write.
func (s *PayoutService) UpdateSchedule(ctx context.Context, tenant valueobject.TenantID, policy valueobject.PayoutPolicy, actor, key string) (valueobject.PayoutPolicy, error) {
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "payout.schedule", "tenant/"+string(tenant)); err != nil {
		return valueobject.PayoutPolicy{}, err
	}
	if err := policy.Validate(); err != nil {
		return valueobject.PayoutPolicy{}, entity.NewError("PAYOUT_POLICY_INVALID", "payout policy is invalid")
	}
	rec := port.IdempotencyRecord{
		Key:         key,
		Fingerprint: Fingerprint(key, string(tenant), string(policy.AssetCode), policy.Version),
		TenantID:    tenant,
	}
	var result valueobject.PayoutPolicy
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			current, err := s.payouts.GetPolicy(ctx, tenant, policy.AssetCode)
			if err != nil {
				return err
			}
			result = current
			return nil
		}
		if err := s.payouts.UpdatePolicy(ctx, policy); err != nil {
			return err
		}
		result = policy
		encoded, err := jsonparser.Marshal(map[string]string{"tenant": string(tenant), "asset": string(policy.AssetCode), "version": policy.Version})
		if err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return valueobject.PayoutPolicy{}, err
	}
	return result, nil
}

// checkEligibility enforces the eligibility policy plus submission shape
// inside the command's UnitOfWork. Ineligible requests fail with
// PAYOUT_BLOCKED reason details and never reach any provider.
func (s *PayoutService) checkEligibility(ctx context.Context, req port.PayoutRequest) error {
	policy, err := s.payouts.GetPolicy(ctx, req.TenantID, req.AssetCode)
	if err != nil {
		return err
	}
	balance, err := s.balances.Execute(ctx, port.BalanceQuery{
		TenantID: req.TenantID, LedgerID: req.LedgerID, AccountID: req.AccountID, AssetCode: req.AssetCode,
	})
	if err != nil {
		return err
	}
	eligibility, err := service.EvaluateEligibility(service.EligibilityInput{
		Policy: policy, AvailableMinor: balance.AvailableMinor, RequestedMinor: req.AmountMinor,
		AssetCode: req.AssetCode, TenantAgeDays: req.TenantAgeDays,
		DestinationVerified: req.DestinationVerified, Method: req.Method, Cursor: balance.Cursor,
	})
	if err != nil {
		return err
	}
	if !eligibility.Eligible {
		return eligibility.BlockedError()
	}
	_, err = service.ValidateSubmission(service.PayoutSubmission{
		MerchantAccount: req.AccountID, PayoutsPayable: s.transitAccount,
		AmountMinor: req.AmountMinor, AssetCode: req.AssetCode, Method: req.Method,
		Reference: service.PayoutReference{IdempotencyKey: req.IdempotencyKey},
	})
	return err
}

// validatePayoutEnvelope checks the payout command envelope.
func validatePayoutEnvelope(req port.PayoutRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(req.AccountID.String()) == "" {
		return entity.NewError("PAYOUT_ACCOUNT_REQUIRED", "payout requires an account id")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_PAYOUT_AMOUNT", "payout amount must be positive")
	}
	if strings.TrimSpace(string(req.AssetCode)) == "" {
		return entity.NewError("PAYOUT_ASSET_REQUIRED", "payout requires an asset code")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "payout actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payout requires an idempotency key")
	}
	return nil
}

// payoutPayload builds the §10-shaped webhook payload: id plus status.
func payoutPayload(id, status, errorCode string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadStatusKey: status, payloadErrorKey: errorCode})
	return encoded
}

// decodePayoutResult restores a replayed response; corrupt records fail loudly.
func decodePayoutResult(response []byte) (port.PayoutResult, error) {
	var result port.PayoutResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.PayoutResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.PayoutID == "" {
		return port.PayoutResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}

// TopupServiceParams carries dependencies for TopupService.
type TopupServiceParams struct {
	UoW    port.UnitOfWork
	Topups TopupStore
	Clock  port.Clock
	IDs    port.IDGenerator
	Authz  port.Authorizer
}

// TopupService validates top-ups against verified instruments and tracks
// funding state. Settlement events (succeeded/failed) come from the
// bank-settlement path (E10/E14); creation stages no webhook.
type TopupService struct {
	uow    port.UnitOfWork
	topups TopupStore
	clock  port.Clock
	ids    port.IDGenerator
	authz  port.Authorizer
}

// NewTopupService constructs a TopupService with the supplied dependencies.
func NewTopupService(params TopupServiceParams) *TopupService {
	return &TopupService{
		uow:    params.UoW,
		topups: params.Topups,
		clock:  params.Clock,
		ids:    params.IDs,
		authz:  params.Authz,
	}
}

// CreateTopup validates the instrument and persists PENDING funding. Strong write.
func (s *TopupService) CreateTopup(ctx context.Context, req port.TopupRequest) (port.TopupResult, error) {
	if _, err := service.ValidateTopup(service.TopupRequest{
		TenantID: req.TenantID, LedgerID: req.LedgerID, CreditAccount: req.CreditAccount,
		AmountMinor: req.AmountMinor, AssetCode: req.AssetCode, InstrumentID: req.InstrumentID,
		InstrumentVerified: req.InstrumentVerified, IdempotencyKey: req.IdempotencyKey,
	}); err != nil {
		return port.TopupResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "topup.create", "ledger/"+string(req.LedgerID)); err != nil {
		return port.TopupResult{}, err
	}
	topupID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), int64ToString(req.AmountMinor)),
		TenantID:    req.TenantID,
	}
	var result port.TopupResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeTopupResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if err := s.topups.CreateTopup(ctx, TopupRecord{
			ID: topupID, TenantID: req.TenantID, LedgerID: req.LedgerID, AccountID: req.CreditAccount,
			AmountMinor: req.AmountMinor, AssetCode: req.AssetCode, Status: TopupPending, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = port.TopupResult{TopupID: topupID, Status: TopupPending, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.TopupResult{}, err
	}
	return result, nil
}

// CancelTopup cancels a PENDING top-up. Strong write.
func (s *TopupService) CancelTopup(ctx context.Context, tenant valueobject.TenantID, topupID, actor string) (port.TopupResult, error) {
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "topup.cancel", "ledger/"+topupID); err != nil {
		return port.TopupResult{}, err
	}
	record, err := s.topups.FindTopup(ctx, tenant, topupID)
	if err != nil {
		return port.TopupResult{}, err
	}
	if record.Status != TopupPending {
		return port.TopupResult{}, entity.NewError("TOPUP_STATE_INVALID", "only pending top-ups can be canceled")
	}
	record.Status = TopupCanceled
	if err := s.topups.UpdateTopup(ctx, record); err != nil {
		return port.TopupResult{}, err
	}
	return port.TopupResult{TopupID: record.ID, Status: TopupCanceled}, nil
}

// GetTopup returns one top-up. Strong read.
func (s *TopupService) GetTopup(ctx context.Context, tenant valueobject.TenantID, topupID string) (port.TopupResult, error) {
	record, err := s.topups.FindTopup(ctx, tenant, topupID)
	if err != nil {
		return port.TopupResult{}, err
	}
	return port.TopupResult{TopupID: record.ID, Status: record.Status}, nil
}

// ListTopups returns one tenant's top-ups, newest first (bounded). Strong read.
func (s *TopupService) ListTopups(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.TopupResult, error) {
	records, err := s.topups.ListTopups(ctx, tenant, clampPageLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]port.TopupResult, 0, len(records))
	for _, record := range records {
		out = append(out, port.TopupResult{TopupID: record.ID, Status: record.Status})
	}
	return out, nil
}

// PaymentUseCasesParams carries dependencies for PaymentUseCases composite adapter.
type PaymentUseCasesParams struct {
	Intents *IntentService
	Refunds *RefundService
	Payouts *PayoutService
	Topups  *TopupService
}

// PaymentUseCases routes the aggregate payment face to the focused
// services. One adapter keeps edge wiring single; each service owns its
// stores and rules.
type PaymentUseCases struct {
	intents *IntentService
	refunds *RefundService
	payouts *PayoutService
	topups  *TopupService
}

var _ port.PaymentUseCases = (*PaymentUseCases)(nil)

// NewPaymentUseCases constructs a PaymentUseCases composite adapter.
func NewPaymentUseCases(params PaymentUseCasesParams) *PaymentUseCases {
	return &PaymentUseCases{
		intents: params.Intents,
		refunds: params.Refunds,
		payouts: params.Payouts,
		topups:  params.Topups,
	}
}

// CreateIntent creates a payment intent in REQUIRES_METHOD. Strong write.
func (u *PaymentUseCases) CreateIntent(ctx context.Context, req port.PaymentIntentRequest) (port.PaymentIntentResult, error) {
	return u.intents.CreateIntent(ctx, req)
}

// ConfirmIntent confirms and captures an intent. Strong write.
func (u *PaymentUseCases) ConfirmIntent(ctx context.Context, req port.ConfirmIntentRequest) (port.PaymentIntentResult, error) {
	return u.intents.ConfirmIntent(ctx, req)
}

// CancelIntent cancels an uncaptured intent. Strong write.
func (u *PaymentUseCases) CancelIntent(ctx context.Context, query port.PaymentQuery) (port.PaymentIntentResult, error) {
	return u.intents.CancelIntent(ctx, query)
}

// CreateRefund validates window/amount and links the reversal. Strong write.
func (u *PaymentUseCases) CreateRefund(ctx context.Context, req port.RefundRequest) (port.RefundResult, error) {
	return u.refunds.CreateRefund(ctx, req)
}

// CreatePayout gates on eligibility then stages settlement. Strong write.
func (u *PaymentUseCases) CreatePayout(ctx context.Context, req port.PayoutRequest) (port.PayoutResult, error) {
	return u.payouts.CreatePayout(ctx, req)
}

// CancelPayout cancels a PENDING payout. Strong write.
func (u *PaymentUseCases) CancelPayout(ctx context.Context, query port.PaymentQuery) (port.PayoutResult, error) {
	return u.payouts.CancelPayout(ctx, query)
}

// CreateTopup validates the instrument and stages funding. Strong write.
func (u *PaymentUseCases) CreateTopup(ctx context.Context, req port.TopupRequest) (port.TopupResult, error) {
	return u.topups.CreateTopup(ctx, req)
}

// GetIntent returns one payment intent. Strong read.
func (u *PaymentUseCases) GetIntent(ctx context.Context, query port.PaymentQuery) (port.PaymentIntentResult, error) {
	return u.intents.GetIntent(ctx, query)
}

// ListIntents returns one tenant's intents, newest first (bounded). Strong read.
func (u *PaymentUseCases) ListIntents(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PaymentIntentResult, error) {
	return u.intents.ListIntents(ctx, tenant, limit)
}

// GetRefund returns one refund. Strong read.
func (u *PaymentUseCases) GetRefund(ctx context.Context, query port.PaymentQuery) (port.RefundResult, error) {
	return u.refunds.GetRefund(ctx, query)
}

// ListRefunds returns one tenant's refunds, newest first (bounded). Strong read.
func (u *PaymentUseCases) ListRefunds(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.RefundResult, error) {
	return u.refunds.ListRefunds(ctx, tenant, limit)
}

// GetPayout returns one payout. Strong read.
func (u *PaymentUseCases) GetPayout(ctx context.Context, query port.PaymentQuery) (port.PayoutResult, error) {
	return u.payouts.GetPayout(ctx, query)
}

// ListPayouts returns one tenant's payouts, newest first (bounded). Strong read.
func (u *PaymentUseCases) ListPayouts(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PayoutResult, error) {
	return u.payouts.ListPayouts(ctx, tenant, limit)
}

// GetTopup returns one top-up. Strong read.
func (u *PaymentUseCases) GetTopup(ctx context.Context, tenant valueobject.TenantID, topupID string) (port.TopupResult, error) {
	return u.topups.GetTopup(ctx, tenant, topupID)
}

// ListTopups returns one tenant's top-ups, newest first (bounded). Strong read.
func (u *PaymentUseCases) ListTopups(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.TopupResult, error) {
	return u.topups.ListTopups(ctx, tenant, limit)
}

// decodeTopupResult restores a replayed response; corrupt records fail loudly.
func decodeTopupResult(response []byte) (port.TopupResult, error) {
	var result port.TopupResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.TopupResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.TopupID == "" {
		return port.TopupResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
