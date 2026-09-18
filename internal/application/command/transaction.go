package command

import (
	"context"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// ReverseTransactionRequest reverses one committed posting in full, linked to
// the original. The original stays immutable; the reversal is a new
// opposite-side posting.
type ReverseTransactionRequest struct {
	TenantID       valueobject.TenantID
	PostingID      valueobject.PostingID
	Reason         string
	Actor          string
	IdempotencyKey string
}

// TransactionServiceParams carries dependencies for TransactionService.
type TransactionServiceParams struct {
	Posting  port.PostLedgerPosting
	UoW      port.UnitOfWork
	Postings repository.PostingRepository
	Accounts repository.AccountRepository
	Clock    port.Clock
	IDs      port.IDGenerator
	Authz    port.Authorizer
}

// TransactionService backs generic transaction posting plus linked reversals.
// Plain posts delegate to the core posting use case behind its port (never a
// concrete service); reversals mirror through the posting aggregate inside
// one UnitOfWork.
type TransactionService struct {
	posting  port.PostLedgerPosting
	uow      port.UnitOfWork
	postings repository.PostingRepository
	accounts repository.AccountRepository
	clock    port.Clock
	ids      port.IDGenerator
	authz    port.Authorizer
}

// NewTransactionService constructs a TransactionService with the supplied dependencies.
func NewTransactionService(params TransactionServiceParams) *TransactionService {
	return &TransactionService{
		posting:  params.Posting,
		uow:      params.UoW,
		postings: params.Postings,
		accounts: params.Accounts,
		clock:    params.Clock,
		ids:      params.IDs,
		authz:    params.Authz,
	}
}

// PostTransaction posts one template-approved transaction. Strong write.
func (s *TransactionService) PostTransaction(ctx context.Context, cmd port.PostPostingCommand) (port.PostingResult, error) {
	return s.posting.Execute(ctx, cmd)
}

// ReverseTransaction commits the opposite-side linked correction of one
// committed posting. Strong write.
func (s *TransactionService) ReverseTransaction(ctx context.Context, req ReverseTransactionRequest) (port.PostingResult, error) {
	if err := validateReverseRequest(req); err != nil {
		return port.PostingResult{}, err
	}
	subject := port.Subject{ID: req.Actor, TenantID: req.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "transaction.reverse", "ledger/"+string(req.PostingID)); err != nil {
		return port.PostingResult{}, err
	}
	snapshot, err := s.loadReversalSnapshot(ctx, req)
	if err != nil {
		return port.PostingResult{}, err
	}
	rec := port.IdempotencyRecord{
		Key:         req.IdempotencyKey,
		Fingerprint: Fingerprint(req.IdempotencyKey, string(req.TenantID), string(req.PostingID), req.Reason),
		TenantID:    req.TenantID,
	}
	var result port.PostingResult
	err = s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		return s.reverseInTx(ctx, tx, req, snapshot, rec, &result)
	})
	if err != nil {
		return port.PostingResult{}, err
	}
	return result, nil
}

// loadReversalSnapshot reads the original with its account set and mints
// every identity once before Do.
func (s *TransactionService) loadReversalSnapshot(ctx context.Context, req ReverseTransactionRequest) (reversalSnapshot, error) {
	original, err := s.postings.FindByID(ctx, req.TenantID, req.PostingID)
	if err != nil {
		return reversalSnapshot{}, err
	}
	accounts, err := s.loadOriginalAccounts(ctx, req.TenantID, original)
	if err != nil {
		return reversalSnapshot{}, err
	}
	eventID := s.ids.NewID()
	return reversalSnapshot{
		original: original,
		accounts: accounts,
		eventID:  eventID,
		now:      s.clock.Now().UTC(),
	}, nil
}

// reverseInTx replays or commits the linked correction inside the caller's
// UnitOfWork.
func (s *TransactionService) reverseInTx(ctx context.Context, tx port.Tx, req ReverseTransactionRequest, snapshot reversalSnapshot, rec port.IdempotencyRecord, result *port.PostingResult) error {
	outcome, err := tx.Idempotency().Reserve(ctx, rec)
	if err != nil {
		return err
	}
	if outcome.Replay {
		decoded, err := decodePostingResult(outcome.Response)
		if err != nil {
			return err
		}
		*result = decoded
		return nil
	}
	reversed, err := aggregate.ReversePosting(aggregate.LoadPosting(snapshot.original), snapshot.accounts, aggregate.ReverseParams{
		NewID: "", Reason: req.Reason, Actor: valueobject.UserID(req.Actor),
		EventID: snapshot.eventID, At: snapshot.now,
	})
	if err != nil {
		return err
	}
	stored, err := tx.Postings().Commit(ctx, reversed.Record())
	if err != nil {
		return err
	}
	if err := reversed.AssignID(stored.ID, stored.Entries, snapshot.eventID); err != nil {
		return err
	}
	*result = port.PostingResult{PostingID: stored.ID, TenantID: req.TenantID, LedgerID: snapshot.original.LedgerID, Cursor: tx.Cursor()}
	encoded, err := jsonparser.Marshal(*result)
	if err != nil {
		return err
	}
	if err := tx.Outbox().Append(ctx, port.OutboxFact{
		TenantID:    req.TenantID,
		LedgerID:    snapshot.original.LedgerID,
		EventType:   "transaction.reversed.v1",
		AggregateID: string(stored.ID),
		Payload:     encoded,
		OccurredAt:  snapshot.now,
	}); err != nil {
		return err
	}
	return tx.Idempotency().Complete(ctx, rec.Key, encoded)
}

// reversalSnapshot carries everything minted and loaded once before Do so
// retried callbacks reuse identities and snapshots.
type reversalSnapshot struct {
	original entity.PostingData
	accounts map[valueobject.AccountID]entity.AccountData
	eventID  string
	now      time.Time
}

// validateReverseRequest checks the reversal envelope.
func validateReverseRequest(req ReverseTransactionRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.PostingID.String()) == "" {
		return entity.NewError("POSTING_ID_REQUIRED", "posting id is required")
	}
	if strings.TrimSpace(req.Reason) == "" {
		return entity.NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "transaction actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "transaction requires an idempotency key")
	}
	return nil
}

// loadOriginalAccounts reads the construction snapshot for the original's
// accounts. E07 re-validates under deterministic locks at commit.
func (s *TransactionService) loadOriginalAccounts(ctx context.Context, tenant valueobject.TenantID, original entity.PostingData) (map[valueobject.AccountID]entity.AccountData, error) {
	accounts := make(map[valueobject.AccountID]entity.AccountData, len(original.Entries))
	for _, entry := range original.Entries {
		if _, ok := accounts[entry.AccountID]; ok {
			continue
		}
		account, err := s.accounts.FindByID(ctx, tenant, entry.AccountID)
		if err != nil {
			return nil, err
		}
		accounts[entry.AccountID] = account
	}
	return accounts, nil
}
