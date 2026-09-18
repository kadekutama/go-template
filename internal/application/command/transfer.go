package command

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// Transfer lifecycle states.
const (
	TransferPending   = port.TransferPending
	TransferCompleted = port.TransferCompleted
	TransferFailed    = port.TransferFailed
	TransferCanceled  = port.TransferCanceled
)

// Batch lifecycle states.
const (
	BatchReceived  = port.BatchReceived
	BatchCompleted = port.BatchCompleted
	BatchPartial   = port.BatchPartial
)

// MaxBatchItems bounds batch intake per the epic contract.
const MaxBatchItems = 1000

// BatchCompletedEvent is the batch close webhook (domain-events §3.3).
const BatchCompletedEvent = "transfer.batch.completed.v1"

// TransferRecord is the durable transfer intent: parameters, state, and the
// committed posting link once executed.
type TransferRecord = port.TransferRecord

// BatchRecord is the durable batch intake with its completion state.
type BatchRecord = port.BatchRecord

// BatchItem is one batch member with its execution outcome.
type BatchItem = port.BatchItem

// TransferListFilter pages transfer records with optional bounds.
type TransferListFilter = port.TransferListFilter

// TransferStore is the consumer-owned transfer/batch persistence boundary.
// The schema lands in E07-T10; this interface is the contract it implements.
type TransferStore interface {
	// CreateTransfer persists one transfer intent. Strong write; fails on duplicate ID.
	CreateTransfer(ctx context.Context, record TransferRecord) error
	// FindTransfer returns one transfer by tenant + ID. Strong read.
	FindTransfer(ctx context.Context, tenant valueobject.TenantID, id string) (TransferRecord, error)
	// UpdateTransfer replaces one transfer record. Strong write.
	UpdateTransfer(ctx context.Context, record TransferRecord) error
	// ListTransfers pages transfer records by filter. Point-in-time page.
	ListTransfers(ctx context.Context, filter TransferListFilter) ([]TransferRecord, string, error)
	// CreateBatch persists one batch with its items atomically at intake. Strong write.
	CreateBatch(ctx context.Context, batch BatchRecord, items []BatchItem) error
	// FindBatch returns one batch by tenant + ID. Strong read.
	FindBatch(ctx context.Context, tenant valueobject.TenantID, id string) (BatchRecord, error)
	// UpdateBatchState stores the batch completion state. Strong write.
	UpdateBatchState(ctx context.Context, tenant valueobject.TenantID, id, state string) error
	// UpdateBatchItem stores one item outcome. Strong write.
	UpdateBatchItem(ctx context.Context, tenant valueobject.TenantID, batchID string, index int, status, errorCode string) error
	// ListBatchItems returns every item of one batch. Strong read.
	ListBatchItems(ctx context.Context, tenant valueobject.TenantID, batchID string) ([]BatchItem, error)
}

// TransferService backs immediate, scheduled, and bulk transfers over the
// transfer domain service. Funds come from the strong balance read; postings
// commit through the transfer's own UnitOfWork with durable idempotency.
// TransferServiceParams carries dependencies for TransferService.
type TransferServiceParams struct {
	UoW       port.UnitOfWork
	Accounts  repository.AccountRepository
	Balances  port.GetBalance
	Transfers TransferStore
	Clock     port.Clock
	IDs       port.IDGenerator
	Authz     port.Authorizer
}

// TransferService backs immediate, scheduled, and bulk transfers over the
// transfer domain service. Funds come from the strong balance read; postings
// commit through the transfer's own UnitOfWork with durable idempotency.
type TransferService struct {
	uow       port.UnitOfWork
	accounts  repository.AccountRepository
	balances  port.GetBalance
	transfers TransferStore
	clock     port.Clock
	ids       port.IDGenerator
	authz     port.Authorizer
}

// NewTransferService constructs a TransferService with the supplied dependencies.
func NewTransferService(params TransferServiceParams) *TransferService {
	return &TransferService{
		uow:       params.UoW,
		accounts:  params.Accounts,
		balances:  params.Balances,
		transfers: params.Transfers,
		clock:     params.Clock,
		ids:       params.IDs,
		authz:     params.Authz,
	}
}

var _ port.TransferCommandUseCases = (*TransferService)(nil)

// CreateTransfer accepts an immediate transfer (posts now) or a scheduled one
// (persists PENDING without a funds check). Strong write.
func (s *TransferService) CreateTransfer(ctx context.Context, req port.TransferRequest) (port.TransferResult, error) {
	if err := validateTransferEnvelope(req); err != nil {
		return port.TransferResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "transfer.create", "ledger/"+string(req.LedgerID)); err != nil {
		return port.TransferResult{}, err
	}
	transferID := s.ids.NewID()
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	if !req.ExecuteAt.IsZero() {
		return s.createScheduled(ctx, req, transferID, now)
	}
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: fingerprintTransfer(req),
		TenantID:    req.TenantID,
	}
	var result port.TransferResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeTransferResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		accounts, available, err := s.loadTransferSnapshot(ctx, req)
		if err != nil {
			return err
		}
		postingID, err := s.commitTransferPosting(ctx, tx, req, accounts, available, transferID, eventID, now)
		if err != nil {
			return err
		}
		result = port.TransferResult{TransferID: transferID, Status: TransferCompleted, Cursor: tx.Cursor()}
		return s.completeTransfer(ctx, tx, rec, TransferRecord{
			ID: transferID, TenantID: req.TenantID, LedgerID: req.LedgerID,
			Source: req.Source, Dest: req.Dest, AssetCode: req.AssetCode, AmountMinor: req.AmountMinor,
			Status: TransferCompleted, PostingID: postingID, CreatedAt: now,
		}, "transfer.completed.v1", result, now)
	})
	if err != nil {
		return port.TransferResult{}, err
	}
	return result, nil
}

// ExecuteTransfer runs one PENDING transfer, enforcing funds atomically at
// execution. Terminal records replay their stored outcome; concurrent
// executors serialize on the execution key. Strong write.
func (s *TransferService) ExecuteTransfer(ctx context.Context, tenant valueobject.TenantID, transferID, actor string) (port.TransferResult, error) {
	if strings.TrimSpace(actor) == "" {
		return port.TransferResult{}, entity.NewError("ACTOR_REQUIRED", "transfer actor is required")
	}
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, "transfer.execute", "ledger/"+transferID); err != nil {
		return port.TransferResult{}, err
	}
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         "execute:" + transferID,
		Fingerprint: Fingerprint("execute", transferID),
		TenantID:    tenant,
	}
	var result port.TransferResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		return s.executeInTx(ctx, tx, tenant, transferID, actor, rec, eventID, now, &result)
	})
	if err != nil {
		return port.TransferResult{}, err
	}
	return result, nil
}

// executeInTx replays or runs one pending transfer inside the caller's
// UnitOfWork, recording FAILED with its code instead of losing the outcome.
// Reads load inside, after the replay short-circuit (data-flow §2 order).
func (s *TransferService) executeInTx(ctx context.Context, tx port.Tx, tenant valueobject.TenantID, transferID, actor string, rec port.IdempotencyRecord, eventID string, now time.Time, result *port.TransferResult) error {
	outcome, err := tx.Idempotency().Reserve(ctx, rec)
	if err != nil {
		return err
	}
	if outcome.Replay {
		decoded, err := decodeTransferResult(outcome.Response)
		if err != nil {
			return err
		}
		*result = decoded
		return nil
	}
	record, err := s.transfers.FindTransfer(ctx, tenant, transferID)
	if err != nil {
		return err
	}
	if record.Status != TransferPending && record.Status != TransferCompleted {
		return entity.NewError("TRANSFER_STATE_INVALID", "only pending transfers can execute")
	}
	if record.Status == TransferCompleted {
		*result = port.TransferResult{TransferID: record.ID, Status: record.Status, Cursor: tx.Cursor()}
		return tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(*result))
	}
	accounts, available, err := s.loadTransferSnapshot(ctx, transferRequestOf(record, actor))
	if err != nil {
		return err
	}
	postingID, execErr := s.commitTransferPosting(ctx, tx, transferRequestOf(record, actor), accounts, available, transferID, eventID, now)
	if execErr != nil {
		return s.failExecution(ctx, tx, rec, record, transferID, execErr, now, result)
	}
	*result = port.TransferResult{TransferID: transferID, Status: TransferCompleted, Cursor: tx.Cursor()}
	record.Status = TransferCompleted
	record.PostingID = postingID
	if err := s.transfers.UpdateTransfer(ctx, record); err != nil {
		return err
	}
	if err := tx.Outbox().Append(ctx, transferFact(record, "transfer.completed.v1", now)); err != nil {
		return err
	}
	return tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(*result))
}

// failExecution persists the FAILED record with its stable code, stages the
// failure fact, completes idempotency, and returns the execution error.
func (s *TransferService) failExecution(ctx context.Context, tx port.Tx, rec port.IdempotencyRecord, record TransferRecord, transferID string, execErr error, now time.Time, result *port.TransferResult) error {
	record.Status = TransferFailed
	record.ErrorCode = errorCodeOf(execErr)
	if err := s.transfers.UpdateTransfer(ctx, record); err != nil {
		return err
	}
	*result = port.TransferResult{TransferID: transferID, Status: TransferFailed, Cursor: tx.Cursor()}
	if err := tx.Outbox().Append(ctx, transferFact(record, "transfer.failed.v1", now)); err != nil {
		return err
	}
	if err := tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(*result)); err != nil {
		return err
	}
	return execErr
}

// CancelTransfer cancels a PENDING transfer. Strong write.
func (s *TransferService) CancelTransfer(ctx context.Context, query port.TransferQuery) (port.TransferResult, error) {
	if strings.TrimSpace(query.Actor) == "" {
		return port.TransferResult{}, entity.NewError("ACTOR_REQUIRED", "transfer actor is required")
	}
	subject := port.Subject{ID: query.Actor, TenantID: query.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "transfer.cancel", "ledger/"+query.TransferID); err != nil {
		return port.TransferResult{}, err
	}
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         "cancel:" + query.TransferID,
		Fingerprint: Fingerprint("cancel", query.TransferID),
		TenantID:    query.TenantID,
	}
	var result port.TransferResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeTransferResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		current, err := s.transfers.FindTransfer(ctx, query.TenantID, query.TransferID)
		if err != nil {
			return err
		}
		if current.Status != TransferPending {
			return entity.NewError("TRANSFER_STATE_INVALID", "only pending transfers can be canceled")
		}
		current.Status = TransferCanceled
		if err := s.transfers.UpdateTransfer(ctx, current); err != nil {
			return err
		}
		result = port.TransferResult{TransferID: current.ID, Status: TransferCanceled, Cursor: tx.Cursor()}
		if err := tx.Outbox().Append(ctx, transferFact(current, "transfer.canceled.v1", now)); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(result))
	})
	if err != nil {
		return port.TransferResult{}, err
	}
	return result, nil
}

// createScheduled persists a PENDING transfer without any funds check. Funds
// and posting happen atomically at ExecuteTransfer.
func (s *TransferService) createScheduled(ctx context.Context, req port.TransferRequest, transferID string, now time.Time) (port.TransferResult, error) {
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: fingerprintTransfer(req),
		TenantID:    req.TenantID,
	}
	var result port.TransferResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeTransferResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		result = port.TransferResult{TransferID: transferID, Status: TransferPending, Cursor: tx.Cursor()}
		return s.completeTransfer(ctx, tx, rec, TransferRecord{
			ID: transferID, TenantID: req.TenantID, LedgerID: req.LedgerID,
			Source: req.Source, Dest: req.Dest, AssetCode: req.AssetCode, AmountMinor: req.AmountMinor,
			Status: TransferPending, ExecuteAt: req.ExecuteAt, Recurrence: req.Recurrence, CreatedAt: now,
		}, "transfer.created.v1", result, now)
	})
	if err != nil {
		return port.TransferResult{}, err
	}
	return result, nil
}

// commitTransferPosting validates funds and commits the transfer posting
// inside the caller's UnitOfWork.
func (s *TransferService) commitTransferPosting(ctx context.Context, tx port.Tx, req port.TransferRequest, accounts map[valueobject.AccountID]entity.AccountData, available int64, transferID, eventID string, now time.Time) (valueobject.PostingID, error) {
	if err := requireSameAssetTransfer(req, accounts); err != nil {
		return "", err
	}
	lines, err := service.ValidateImmediate(service.TransferRequest{
		TenantID: req.TenantID, LedgerID: req.LedgerID, Source: req.Source, Dest: req.Dest,
		AssetCode: req.AssetCode, AmountMinor: req.AmountMinor, FXRatePresent: req.FXRatePresent,
	}, accounts, available)
	if err != nil {
		return "", err
	}
	entries := BuildPostingEntries([]port.NewEntry{
		{AccountID: lines.DebitAccount, Side: valueobject.DirectionDebit, AmountMinor: lines.AmountMinor, AssetCode: lines.AssetCode},
		{AccountID: lines.CreditAccount, Side: valueobject.DirectionCredit, AmountMinor: lines.AmountMinor, AssetCode: lines.AssetCode},
	}, "")
	posting, err := aggregate.ConstructPosting(aggregate.PostingParams{
		ID: "", TenantID: req.TenantID, LedgerID: req.LedgerID,
		Operation: "transfer", ExternalReference: transferID,
		Entries: entries, Accounts: accounts,
		EffectiveAt: now, RecordedAt: now, EventID: eventID,
	})
	if err != nil {
		return "", err
	}
	stored, err := tx.Postings().Commit(ctx, posting.Record())
	if err != nil {
		return "", err
	}
	if err := posting.AssignID(stored.ID, stored.Entries, eventID); err != nil {
		return "", err
	}
	return stored.ID, nil
}

// requireSameAssetTransfer enforces the slice boundary: this handler posts
// single-asset transfers only. Cross-currency settlement needs linked FX
// lots with provider rates (E10 follow-up); without them a cross-currency
// request would die opaquely inside construction, so it fails here with a
// clear code instead.
func requireSameAssetTransfer(req port.TransferRequest, accounts map[valueobject.AccountID]entity.AccountData) error {
	for _, id := range []valueobject.AccountID{req.Source, req.Dest} {
		if accounts[id].AssetCode != req.AssetCode {
			return entity.NewError("CROSS_CURRENCY_UNSUPPORTED", "cross-currency transfers need FX settlement lots (E10 follow-up)")
		}
	}
	return nil
}

// completeTransfer persists the transfer record, stages its event fact, and
// completes idempotency inside the caller's UnitOfWork. Duplicate IDs fail
// loudly: retried callbacks reuse pre-minted IDs and resolve through replay,
// so a duplicate here signals a genuinely conflicting intent.
func (s *TransferService) completeTransfer(ctx context.Context, tx port.Tx, rec port.IdempotencyRecord, record TransferRecord, eventType string, result port.TransferResult, now time.Time) error {
	if err := s.transfers.CreateTransfer(ctx, record); err != nil {
		return err
	}
	if err := tx.Outbox().Append(ctx, transferFact(record, eventType, now)); err != nil {
		return err
	}
	return tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(result))
}

// loadTransferSnapshot reads the account set plus the authoritative source
// available inside the command's UnitOfWork (after the replay check).
// E07 re-validates under locks at commit.
func (s *TransferService) loadTransferSnapshot(ctx context.Context, req port.TransferRequest) (map[valueobject.AccountID]entity.AccountData, int64, error) {
	accounts := make(map[valueobject.AccountID]entity.AccountData, 2)
	for _, id := range []valueobject.AccountID{req.Source, req.Dest} {
		if _, ok := accounts[id]; ok {
			continue
		}
		account, err := s.accounts.FindByID(ctx, req.TenantID, id)
		if err != nil {
			return nil, 0, err
		}
		accounts[id] = account
	}
	balance, err := s.balances.Execute(ctx, port.BalanceQuery{
		TenantID: req.TenantID, LedgerID: req.LedgerID, AccountID: req.Source, AssetCode: req.AssetCode,
	})
	if err != nil {
		return nil, 0, err
	}
	return accounts, balance.AvailableMinor, nil
}

// validateTransferEnvelope checks the transfer command envelope.
func validateTransferEnvelope(req port.TransferRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(string(req.Source)) == "" || strings.TrimSpace(string(req.Dest)) == "" {
		return entity.NewError("TRANSFER_ACCOUNT_REQUIRED", "transfer requires source and destination accounts")
	}
	if strings.TrimSpace(string(req.AssetCode)) == "" {
		return entity.NewError("TRANSFER_ASSET_REQUIRED", "transfer requires an asset code")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "transfer actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "transfer requires an idempotency key")
	}
	return nil
}

// fingerprintTransfer canonicalizes a transfer command for idempotency.
func fingerprintTransfer(req port.TransferRequest) string {
	return Fingerprint(req.IdempotencyKey, string(req.TenantID), string(req.LedgerID),
		string(req.Source), string(req.Dest), string(req.AssetCode), int64ToString(req.AmountMinor),
		req.ExecuteAt.UTC().Format(time.RFC3339Nano), req.Recurrence)
}

// transferRequestOf rebuilds the creation command for a stored record.
func transferRequestOf(record TransferRecord, actor string) port.TransferRequest {
	return port.TransferRequest{
		TenantID: record.TenantID, LedgerID: record.LedgerID, Source: record.Source, Dest: record.Dest,
		AssetCode: record.AssetCode, AmountMinor: record.AmountMinor,
		ExecuteAt: record.ExecuteAt, Recurrence: record.Recurrence, Actor: actor,
	}
}

// transferFact builds the outbox fact for a transfer state change.
func transferFact(record TransferRecord, eventType string, now time.Time) port.OutboxFact {
	return port.OutboxFact{
		TenantID:    record.TenantID,
		LedgerID:    record.LedgerID,
		EventType:   eventType,
		AggregateID: record.ID,
		OccurredAt:  now,
	}
}

// decodeTransferResult restores a replayed response; corrupt records fail loudly.
func decodeTransferResult(response []byte) (port.TransferResult, error) {
	var result port.TransferResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.TransferResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.TransferID == "" {
		return port.TransferResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}

// mustEncodeTransfer encodes a result for idempotency storage; results are
// plain strings and encoding cannot fail.
func mustEncodeTransfer(result port.TransferResult) []byte {
	encoded, err := jsonparser.Marshal(result)
	if err != nil {
		panic("transfer: result encoding must not fail: " + err.Error())
	}
	return encoded
}

// errorCodeOf extracts the stable code for per-item failure records,
// unwrapping joined/wrapped domain errors.
func errorCodeOf(err error) string {
	var domainErr *entity.Error
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return "INTERNAL_ERROR"
}

// CreateBatchTransfer validates and persists a batch atomically at intake,
// then executes items independently: sibling failure never rolls back and
// the batch completes PARTIAL instead of failing. Strong write.
func (s *TransferService) CreateBatchTransfer(ctx context.Context, req port.BatchTransferRequest) (port.BatchTransferResult, error) {
	if err := validateBatchEnvelope(req); err != nil {
		return port.BatchTransferResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "transfer.batch", "ledger/"+string(req.LedgerID)); err != nil {
		return port.BatchTransferResult{}, err
	}
	batchID := s.ids.NewID()
	now := s.clock.Now().UTC()
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: fingerprintBatch(req),
		TenantID:    req.TenantID,
	}
	items := make([]BatchItem, 0, len(req.Items))
	transferIDs := make([]string, 0, len(req.Items))
	for i := range req.Items {
		transferIDs = append(transferIDs, s.ids.NewID())
		items = append(items, BatchItem{BatchID: batchID, Index: i, TransferID: transferIDs[i], Status: TransferPending})
	}
	var result port.BatchTransferResult
	replayed := false
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		intaked, wasReplay, err := s.intakeBatch(ctx, tx, req, batchID, items, transferIDs, rec, now)
		if err != nil {
			return err
		}
		result = intaked
		replayed = wasReplay
		return nil
	})
	if err != nil {
		return port.BatchTransferResult{}, err
	}
	if replayed {
		return s.convergeBatch(ctx, req.TenantID, req.LedgerID, req.Actor, result.BatchID, now, result)
	}
	requests := make([]port.TransferRequest, 0, len(req.Items))
	for _, item := range req.Items {
		item.Actor = req.Actor
		requests = append(requests, item)
	}
	indices := make([]int, 0, len(req.Items))
	for i := range req.Items {
		indices = append(indices, i)
	}
	return s.executeBatchItems(ctx, req.TenantID, req.LedgerID, req.Actor, result.BatchID, requests, transferIDs, indices, result, now)
}

// intakeBatch reserves intake idempotency and, unless replaying, persists
// the batch with its PENDING transfer intents plus the received fact.
func (s *TransferService) intakeBatch(ctx context.Context, tx port.Tx, req port.BatchTransferRequest, batchID string, items []BatchItem, transferIDs []string, rec port.IdempotencyRecord, now time.Time) (port.BatchTransferResult, bool, error) {
	var result port.BatchTransferResult
	outcome, err := tx.Idempotency().Reserve(ctx, rec)
	if err != nil {
		return result, false, err
	}
	if outcome.Replay {
		decoded, err := decodeBatchResult(outcome.Response)
		if err != nil {
			return result, false, err
		}
		return decoded, true, nil
	}
	if err := s.transfers.CreateBatch(ctx, BatchRecord{
		ID: batchID, TenantID: req.TenantID, LedgerID: req.LedgerID, State: BatchReceived, CreatedAt: now,
	}, items); err != nil {
		return result, false, err
	}
	for i := range req.Items {
		if err := s.transfers.CreateTransfer(ctx, TransferRecord{
			ID: transferIDs[i], TenantID: req.TenantID, LedgerID: req.LedgerID,
			Source: req.Items[i].Source, Dest: req.Items[i].Dest,
			AssetCode: req.Items[i].AssetCode, AmountMinor: req.Items[i].AmountMinor,
			Status: TransferPending, CreatedAt: now,
		}); err != nil {
			return result, false, err
		}
	}
	result = port.BatchTransferResult{BatchID: batchID, TotalItems: len(items), AcceptedItems: len(items), Cursor: tx.Cursor()}
	encoded, err := jsonparser.Marshal(result)
	if err != nil {
		return result, false, err
	}
	if err := tx.Outbox().Append(ctx, port.OutboxFact{
		TenantID: req.TenantID, LedgerID: req.LedgerID, EventType: "transfer.batch.received.v1",
		AggregateID: batchID, Payload: encoded, OccurredAt: now,
	}); err != nil {
		return result, false, err
	}
	if err := tx.Idempotency().Complete(ctx, rec.Key, encoded); err != nil {
		return result, false, err
	}
	return result, false, nil
}

// convergeBatch resumes an intake-replayed batch: item requests rebuild from
// their stored intents and pending items execute with the original
// batchID:index keys. Completed work replays safely, so convergence is
// idempotent no matter when the first attempt died.
func (s *TransferService) convergeBatch(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, actor, batchID string, now time.Time, result port.BatchTransferResult) (port.BatchTransferResult, error) {
	items, err := s.transfers.ListBatchItems(ctx, tenant, batchID)
	if err != nil {
		return port.BatchTransferResult{}, err
	}
	sortBatchItems(items)
	requests := make([]port.TransferRequest, 0, len(items))
	transferIDs := make([]string, 0, len(items))
	indices := make([]int, 0, len(items))
	for _, item := range items {
		record, err := s.transfers.FindTransfer(ctx, tenant, item.TransferID)
		if err != nil {
			return port.BatchTransferResult{}, err
		}
		if record.Status != TransferPending {
			continue
		}
		requests = append(requests, transferRequestOf(record, actor))
		transferIDs = append(transferIDs, record.ID)
		indices = append(indices, item.Index)
	}
	if len(requests) == 0 {
		return s.closeBatch(ctx, tenant, ledger, batchID, now, result)
	}
	return s.executeBatchItems(ctx, tenant, ledger, actor, batchID, requests, transferIDs, indices, result, now)
}

// sortBatchItems orders batch members by index for deterministic execution.
func sortBatchItems(items []BatchItem) {
	sort.Slice(items, func(i, j int) bool { return items[i].Index < items[j].Index })
}

// executeBatchItems runs every item in its own UnitOfWork keyed by the
// item's original batch index, records per-item outcomes, and closes the
// batch COMPLETED or PARTIAL. Item errors are recorded, never propagated.
func (s *TransferService) executeBatchItems(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, actor, batchID string, items []port.TransferRequest, transferIDs []string, indices []int, result port.BatchTransferResult, now time.Time) (port.BatchTransferResult, error) {
	for i := range items {
		items[i].Actor = actor
		itemErr := s.runBatchItem(ctx, tenant, ledger, batchID, indices[i], transferIDs[i], items[i], now)
		status := TransferCompleted
		code := ""
		if itemErr != nil {
			status = TransferFailed
			code = errorCodeOf(itemErr)
		}
		if err := s.transfers.UpdateBatchItem(ctx, tenant, batchID, indices[i], status, code); err != nil {
			return port.BatchTransferResult{}, err
		}
	}
	return s.closeBatch(ctx, tenant, ledger, batchID, now, result)
}

// closeBatch recomputes batch state from item outcomes, persists it, and
// stages the close webhook.
func (s *TransferService) closeBatch(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, batchID string, now time.Time, result port.BatchTransferResult) (port.BatchTransferResult, error) {
	items, err := s.transfers.ListBatchItems(ctx, tenant, batchID)
	if err != nil {
		return port.BatchTransferResult{}, err
	}
	state := BatchCompleted
	for _, item := range items {
		if item.Status != TransferCompleted {
			state = BatchPartial
			break
		}
	}
	if err := s.transfers.UpdateBatchState(ctx, tenant, batchID, state); err != nil {
		return port.BatchTransferResult{}, err
	}
	if err := s.stageBatchCompleted(ctx, tenant, ledger, batchID, state, now); err != nil {
		return port.BatchTransferResult{}, err
	}
	return result, nil
}

// stageBatchCompleted stages the batch close webhook once per batch,
// idempotent on its own key so converging replays never duplicate it.
func (s *TransferService) stageBatchCompleted(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, batchID, state string, now time.Time) error {
	rec := port.IdempotencyRecord{
		Key:         "batch-complete:" + batchID,
		Fingerprint: Fingerprint("batch-complete", batchID, state),
		TenantID:    tenant,
	}
	return s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			return nil
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID: tenant, LedgerID: ledger, EventType: BatchCompletedEvent,
			AggregateID: batchID, Payload: batchCompletedPayload(batchID, state), OccurredAt: now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, []byte(`{"batch_id":"`+batchID+`"}`))
	})
}

// batchCompletedPayload builds the §3.3-shaped close payload: id plus state.
func batchCompletedPayload(batchID, state string) []byte {
	encoded, _ := jsonparser.Marshal(map[string]string{payloadIDKey: batchID, payloadStatusKey: state})
	return encoded
}

// runBatchItem runs one batch member through the immediate-transfer path
// with its batchID:index idempotency key. The intake-persisted PENDING
// record is updated in place, linking the committed posting on success.
func (s *TransferService) runBatchItem(ctx context.Context, tenant valueobject.TenantID, ledger valueobject.LedgerID, batchID string, index int, transferID string, item port.TransferRequest, now time.Time) error {
	item.IdempotencyKey = batchID + ":" + int64ToString(int64(index))
	return s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		rec := port.IdempotencyRecord{
			Key:         item.IdempotencyKey,
			Fingerprint: fingerprintTransfer(item),
			TenantID:    tenant,
		}
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			return nil
		}
		accounts, available, err := s.loadTransferSnapshot(ctx, item)
		if err != nil {
			return s.failBatchItem(ctx, transferID, item, err)
		}
		postingID, err := s.commitTransferPosting(ctx, tx, item, accounts, available, transferID, s.ids.NewID(), now)
		if err != nil {
			return s.failBatchItem(ctx, transferID, item, err)
		}
		if err := s.transfers.UpdateTransfer(ctx, TransferRecord{
			ID: transferID, TenantID: tenant, LedgerID: ledger,
			Source: item.Source, Dest: item.Dest, AssetCode: item.AssetCode, AmountMinor: item.AmountMinor,
			Status: TransferCompleted, PostingID: postingID, CreatedAt: now,
		}); err != nil {
			return err
		}
		itemResult := port.TransferResult{TransferID: transferID, Status: TransferCompleted, Cursor: tx.Cursor()}
		if err := tx.Outbox().Append(ctx, transferFact(TransferRecord{
			ID: transferID, TenantID: tenant, LedgerID: ledger,
			Source: item.Source, Dest: item.Dest, AssetCode: item.AssetCode, AmountMinor: item.AmountMinor,
		}, "transfer.completed.v1", now)); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, mustEncodeTransfer(itemResult))
	})
}

// failBatchItem records one item's failure with its stable code.
func (s *TransferService) failBatchItem(ctx context.Context, transferID string, item port.TransferRequest, cause error) error {
	record, err := s.transfers.FindTransfer(ctx, item.TenantID, transferID)
	if err != nil {
		return err
	}
	record.Status = TransferFailed
	record.ErrorCode = errorCodeOf(cause)
	if err := s.transfers.UpdateTransfer(ctx, record); err != nil {
		return err
	}
	return cause
}

// validateBatchEnvelope rejects over-limit, empty, and mixed-tenant batches
// before intake.
func validateBatchEnvelope(req port.BatchTransferRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if len(req.Items) == 0 {
		return entity.NewError("BATCH_EMPTY", "batch requires at least one item")
	}
	if len(req.Items) > MaxBatchItems {
		return entity.NewError("BATCH_TOO_LARGE", "batch exceeds 1000 items")
	}
	for _, item := range req.Items {
		if item.TenantID != req.TenantID {
			return entity.NewError("BATCH_TENANT_MISMATCH", "batch items must share the batch tenant")
		}
		if err := validateTransferEnvelope(item); err != nil {
			return err
		}
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "transfer actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "transfer requires an idempotency key")
	}
	return nil
}

// fingerprintBatch canonicalizes batch intake for idempotency.
func fingerprintBatch(req port.BatchTransferRequest) string {
	parts := []string{req.IdempotencyKey, string(req.TenantID), string(req.LedgerID), int64ToString(int64(len(req.Items)))}
	for _, item := range req.Items {
		parts = append(parts, string(item.Source), string(item.Dest), string(item.AssetCode), int64ToString(item.AmountMinor))
	}
	return Fingerprint(parts...)
}

// decodeBatchResult restores a replayed intake response; corrupt records fail loudly.
func decodeBatchResult(response []byte) (port.BatchTransferResult, error) {
	var result port.BatchTransferResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.BatchTransferResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.BatchID == "" {
		return port.BatchTransferResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
