package command

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Refund webhook event names (api-contracts §10).
const (
	EventRefundCreated   = "refund.created.v1"
	EventRefundSucceeded = "refund.succeeded.v1"
	EventRefundFailed    = "refund.failed.v1"
)

// Refund lifecycle states.
const (
	RefundPending   = "PENDING"
	RefundSucceeded = "SUCCEEDED"
	RefundFailed    = "FAILED"
)

// RefundRecord is the durable refund with its reversal linkage to the
// original payment.
type RefundRecord struct {
	ID          string
	TenantID    valueobject.TenantID
	OriginalTxn string
	AmountMinor int64
	Status      string
	ErrorCode   string
	CreatedAt   time.Time
}

// RefundStore is the consumer-owned refund persistence boundary. The schema
// lands in E07-T10; this interface is the contract it implements.
type RefundStore interface {
	// CreateRefund persists one refund. Strong write; fails on duplicate ID.
	CreateRefund(ctx context.Context, record RefundRecord) error
	// FindRefund returns one refund by tenant + ID. Strong read.
	FindRefund(ctx context.Context, tenant valueobject.TenantID, id string) (RefundRecord, error)
	// SumPriorRefunds totals completed refunds against one original. Strong read.
	SumPriorRefunds(ctx context.Context, tenant valueobject.TenantID, originalTxn string) (int64, error)
	// ListRefunds returns one tenant's refunds, newest first (bounded by the adapter). Strong read.
	ListRefunds(ctx context.Context, tenant valueobject.TenantID, limit int) ([]RefundRecord, error)
}

// SettlementAccounts carries the operator-configured journal accounts for
// refund acceptance/settlement legs (ledger-core §6.4).
type SettlementAccounts struct {
	MerchantPayable valueobject.AccountID
	RefundsPayable  valueobject.AccountID
	CashAccount     valueobject.AccountID
}

// RefundServiceParams carries dependencies for RefundService.
type RefundServiceParams struct {
	UoW        port.UnitOfWork
	Refunds    RefundStore
	Intents    IntentStore
	Processor  port.PaymentProcessor
	Accounts   repository.AccountRepository
	Settlement SettlementAccounts
	WindowDays int
	Clock      port.Clock
	IDs        port.IDGenerator
	Authz      port.Authorizer
}

// RefundService validates refunds against window/amount rules with explicit
// fee policy, executes through the processor, and links the reversal.
// Processor calls happen OUTSIDE UnitOfWork callbacks.
type RefundService struct {
	uow        port.UnitOfWork
	refunds    RefundStore
	intents    IntentStore
	processor  port.PaymentProcessor
	accounts   repository.AccountRepository
	settlement SettlementAccounts
	windowDays int
	clock      port.Clock
	ids        port.IDGenerator
	authz      port.Authorizer
}

// NewRefundService constructs a RefundService with the supplied dependencies.
func NewRefundService(params RefundServiceParams) *RefundService {
	return &RefundService{
		uow:        params.UoW,
		refunds:    params.Refunds,
		intents:    params.Intents,
		processor:  params.Processor,
		accounts:   params.Accounts,
		settlement: params.Settlement,
		windowDays: params.WindowDays,
		clock:      params.Clock,
		ids:        params.IDs,
		authz:      params.Authz,
	}
}

// CreateRefund validates, charges back through the processor, and persists
// the linked refund. Processor failure persists FAILED with a failed fact.
// Strong write.
func (s *RefundService) CreateRefund(ctx context.Context, req port.RefundRequest) (port.RefundResult, error) {
	if err := validateRefundEnvelope(req); err != nil {
		return port.RefundResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "refund.create", "payment/"+req.OriginalTxn); err != nil {
		return port.RefundResult{}, err
	}
	original, err := s.validateRefundable(ctx, req)
	if err != nil {
		return port.RefundResult{}, err
	}
	charged, err := s.processor.RefundCharge(ctx, port.RefundChargeRequest{
		TenantID: req.TenantID, ProviderID: original.ProviderID,
		AmountMinor: req.AmountMinor, IdempotencyKey: req.IdempotencyKey, Timeout: processorTimeout,
	})
	_ = charged
	now := s.clock.Now().UTC()
	if err != nil {
		return s.persistRefund(ctx, req, now, RefundFailed, errorCodeOf(err), EventRefundFailed)
	}
	return s.persistRefund(ctx, req, now, RefundSucceeded, "", EventRefundSucceeded)
}

// GetRefund returns one refund. Strong read.
func (s *RefundService) GetRefund(ctx context.Context, query port.PaymentQuery) (port.RefundResult, error) {
	record, err := s.refunds.FindRefund(ctx, valueobject.TenantID(query.TenantID), query.ID)
	if err != nil {
		return port.RefundResult{}, err
	}
	return port.RefundResult{RefundID: record.ID, OriginalID: record.OriginalTxn, Status: record.Status}, nil
}

// ListRefunds returns one tenant's refunds, newest first (bounded). Strong read.
func (s *RefundService) ListRefunds(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.RefundResult, error) {
	records, err := s.refunds.ListRefunds(ctx, tenant, clampPageLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]port.RefundResult, 0, len(records))
	for _, record := range records {
		out = append(out, port.RefundResult{RefundID: record.ID, OriginalID: record.OriginalTxn, Status: record.Status})
	}
	return out, nil
}

// persistRefund stores one refund outcome with its webhook fact in a single
// UnitOfWork keyed for idempotent retry.
func (s *RefundService) persistRefund(ctx context.Context, req port.RefundRequest, now time.Time, status, errorCode, eventType string) (port.RefundResult, error) {
	refundID := s.ids.NewID()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), req.OriginalTxn, int64ToString(req.AmountMinor)),
		TenantID:    req.TenantID,
	}
	var result port.RefundResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeRefundResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		if status == RefundSucceeded {
			// Atomic recheck: a concurrent refund may have landed between
			// validation and the processor call. Failed attempts always
			// persist (they describe attempts, never balances).
			if _, err := s.validateRefundable(ctx, req); err != nil {
				return err
			}
		}
		if err := s.refunds.CreateRefund(ctx, RefundRecord{
			ID: refundID, TenantID: req.TenantID, OriginalTxn: req.OriginalTxn,
			AmountMinor: req.AmountMinor, Status: status, ErrorCode: errorCode, CreatedAt: now,
		}); err != nil {
			return err
		}
		result = port.RefundResult{RefundID: refundID, OriginalID: req.OriginalTxn, Status: status, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: req.TenantID, EventType: eventType,
			AggregateID: refundID, Payload: refundPayload(refundID, req.OriginalTxn, status, errorCode), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.RefundResult{}, err
	}
	if status == RefundFailed {
		return result, entity.NewError("REFUND_PROCESSOR_FAILED", "processor refund failed")
	}
	return result, nil
}

// validateRefundable reloads the original, prior refunds, and settlement
// accounts, then enforces window/amount rules. It runs before the processor
// call (fail fast, never charge invalid refunds) and again inside persist
// (atomic recheck against concurrent refunds).
func (s *RefundService) validateRefundable(ctx context.Context, req port.RefundRequest) (IntentRecord, error) {
	original, err := s.intents.FindIntent(ctx, req.TenantID, req.OriginalTxn)
	if err != nil {
		return IntentRecord{}, err
	}
	prior, err := s.refunds.SumPriorRefunds(ctx, req.TenantID, req.OriginalTxn)
	if err != nil {
		return IntentRecord{}, err
	}
	accounts, err := s.loadRefundAccounts(ctx, req.TenantID)
	if err != nil {
		return IntentRecord{}, err
	}
	if _, err := service.ValidateRefund(service.RefundRequest{
		OriginalPostingID:   valueobject.PostingID(req.OriginalTxn),
		OriginalAmountMinor: original.AmountMinor,
		PriorRefundedMinor:  prior,
		AmountMinor:         req.AmountMinor,
		OriginalAt:          original.CreatedAt,
		Now:                 s.clock.Now().UTC(),
		WindowDays:          s.effectiveWindowDays(),
		MerchantPayable:     s.settlement.MerchantPayable,
		RefundsPayable:      s.settlement.RefundsPayable,
		CashAccount:         s.settlement.CashAccount,
		AssetCode:           original.AssetCode,
		FeePolicy:           service.FeeRefundPolicy{},
	}, true, accounts); err != nil {
		return IntentRecord{}, err
	}
	return original, nil
}

func (s *RefundService) loadRefundAccounts(ctx context.Context, tenant valueobject.TenantID) (map[valueobject.AccountID]entity.AccountData, error) {
	accounts := make(map[valueobject.AccountID]entity.AccountData, 3)
	for _, id := range []valueobject.AccountID{s.settlement.MerchantPayable, s.settlement.RefundsPayable, s.settlement.CashAccount} {
		if _, ok := accounts[id]; ok {
			continue
		}
		account, err := s.accounts.FindByID(ctx, tenant, id)
		if err != nil {
			return nil, err
		}
		accounts[id] = account
	}
	return accounts, nil
}

// effectiveWindowDays returns the configured refund window or the domain default.
func (s *RefundService) effectiveWindowDays() int {
	if s.windowDays <= 0 {
		return service.DefaultRefundWindowDays
	}
	return s.windowDays
}

// validateRefundEnvelope checks the refund command envelope.
func validateRefundEnvelope(req port.RefundRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.OriginalTxn) == "" {
		return entity.NewError("REFUND_ORIGINAL_REQUIRED", "refund requires the original transaction")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_REFUND_AMOUNT", "refund amount must be positive")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "refund actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "refund requires an idempotency key")
	}
	return nil
}

// refundPayload builds the §10-shaped webhook payload: id, original, status.
func refundPayload(id, original, status, errorCode string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: id, payloadOriginalKey: original, payloadStatusKey: status, payloadErrorKey: errorCode})
	return encoded
}

// decodeRefundResult restores a replayed response; corrupt records fail loudly.
func decodeRefundResult(response []byte) (port.RefundResult, error) {
	var result port.RefundResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.RefundResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.RefundID == "" {
		return port.RefundResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
