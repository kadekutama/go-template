// Package command owns the core posting use case: the first vertical slice
// from validated command to durably committed posting. Extended
// bounded-context commands live in E06-T02–T05/T11; they compose the same
// plumbing (shared authz/idempotency stages) and ports defined in E06-T01/T06/T12.
package command

import (
	"context"
	"strconv"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// PostingServiceParams carries dependencies for PostingService.
type PostingServiceParams struct {
	UoW      port.UnitOfWork
	Accounts repository.AccountRepository
	Clock    port.Clock
	IDs      port.IDGenerator
	Authz    port.Authorizer
}

// PostingService is the restricted core posting use case. It authorizes the
// command, reserves durable idempotency, constructs through the posting
// aggregate on a loaded account snapshot, and commits posting + outbox fact +
// idempotency result in exactly one UnitOfWork.
type PostingService struct {
	uow      port.UnitOfWork
	accounts repository.AccountRepository
	clock    port.Clock
	ids      port.IDGenerator
	authz    port.Authorizer
}

// NewPostingService constructs a PostingService with the supplied dependencies.
func NewPostingService(params PostingServiceParams) *PostingService {
	return &PostingService{
		uow:      params.UoW,
		accounts: params.Accounts,
		clock:    params.Clock,
		ids:      params.IDs,
		authz:    params.Authz,
	}
}

var _ port.PostLedgerPosting = (*PostingService)(nil)

// Execute posts one template-approved posting atomically. Identical
// resubmission returns the original response; fingerprint conflicts fail
// without executing; unknown commit outcomes surface as errors resolved by
// identical resubmission, never blind retry.
//
// Reads load inside Do after the replay check (data-flow §2 order): replays
// skip them, and first executions read within the atomic unit.
func (s *PostingService) Execute(ctx context.Context, cmd port.PostPostingCommand) (port.PostingResult, error) {
	if err := validatePostingCommand(cmd); err != nil {
		return port.PostingResult{}, err
	}
	subject := port.Subject{ID: cmd.Actor, TenantID: cmd.TenantID}
	if err := RequireAuthz(ctx, s.authz, subject, "ledger.post", "ledger/"+string(cmd.LedgerID)); err != nil {
		return port.PostingResult{}, err
	}
	// Mint identities once before Do so retried callbacks reuse them and can
	// never double-mint; duplicate posting IDs fail at commit for replay
	// resolution.
	postingID := valueobject.PostingID(s.ids.NewID())
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	entries := BuildPostingEntries(cmd.Entries, postingID, s.ids)
	rec := port.IdempotencyRecord{
		Key:         cmd.IdempotencyKey,
		Fingerprint: fingerprintPostingCommand(cmd),
		TenantID:    cmd.TenantID,
	}
	var result port.PostingResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodePostingResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		accounts, err := s.loadAccounts(ctx, cmd)
		if err != nil {
			return err
		}
		posting, err := aggregate.ConstructPosting(aggregate.PostingParams{
			ID:                postingID,
			TenantID:          cmd.TenantID,
			LedgerID:          cmd.LedgerID,
			Operation:         cmd.Operation,
			ExternalReference: cmd.ExternalReference,
			Description:       cmd.Description,
			Entries:           entries,
			Accounts:          accounts,
			EffectiveAt:       now,
			RecordedAt:        now,
			EventID:           eventID,
		})
		if err != nil {
			return err
		}
		if err := tx.Postings().Commit(ctx, posting.Record()); err != nil {
			return err
		}
		result = port.PostingResult{
			PostingID: posting.ID(),
			TenantID:  cmd.TenantID,
			LedgerID:  cmd.LedgerID,
			Cursor:    tx.Cursor(),
		}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		if err := tx.Outbox().Append(ctx, port.OutboxFact{
			TenantID:    cmd.TenantID,
			LedgerID:    cmd.LedgerID,
			EventType:   "transaction.posted.v1",
			AggregateID: string(posting.ID()),
			Payload:     encoded,
			OccurredAt:  now,
		}); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.PostingResult{}, err
	}
	return result, nil
}

// validatePostingCommand checks the command envelope. Line-level rules
// (operation template, entry count/shape/balance) belong to the domain and
// surface from ConstructPosting unchanged.
func validatePostingCommand(cmd port.PostPostingCommand) error {
	if strings.TrimSpace(cmd.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(cmd.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(cmd.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "posting actor is required")
	}
	if strings.TrimSpace(cmd.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "posting requires an idempotency key")
	}
	return nil
}

// loadAccounts reads the construction account set inside the command's
// UnitOfWork (after the replay check). E07 re-validates scope/status under
// deterministic account locks at commit (data-flow steps 3–5).
func (s *PostingService) loadAccounts(ctx context.Context, cmd port.PostPostingCommand) (map[valueobject.AccountID]entity.AccountData, error) {
	accounts := make(map[valueobject.AccountID]entity.AccountData, len(cmd.Entries))
	for _, line := range cmd.Entries {
		if _, ok := accounts[line.AccountID]; ok {
			continue
		}
		account, err := s.accounts.FindByID(ctx, cmd.TenantID, line.AccountID)
		if err != nil {
			return nil, err
		}
		accounts[line.AccountID] = account
	}
	return accounts, nil
}

// BuildPostingEntries maps request lines to domain entries with minted IDs
// and deterministic 1-based per-account positions (domain requires
// AccountSeq ≥ 1; durable global sequencing is assigned at commit by
// E07-T01 — see the E06-T13 packet). Shared with transfer construction.
func BuildPostingEntries(lines []port.NewEntry, postingID valueobject.PostingID, ids port.IDGenerator) []entity.Entry {
	positions := make(map[valueobject.AccountID]int64, len(lines))
	entries := make([]entity.Entry, 0, len(lines))
	for _, line := range lines {
		positions[line.AccountID]++
		entries = append(entries, entity.Entry{
			ID:          valueobject.EntryID(ids.NewID()),
			PostingID:   postingID,
			AccountID:   line.AccountID,
			Side:        line.Side,
			AmountMinor: line.AmountMinor,
			AssetCode:   line.AssetCode,
			AccountSeq:  positions[line.AccountID],
		})
	}
	return entries
}

// fingerprintPostingCommand canonicalizes the command into a stable
// fingerprint over key scope, operation, and every line.
func fingerprintPostingCommand(cmd port.PostPostingCommand) string {
	parts := []string{cmd.IdempotencyKey, string(cmd.TenantID), string(cmd.LedgerID), cmd.Operation, cmd.ExternalReference}
	for _, line := range cmd.Entries {
		parts = append(parts, string(line.AccountID), string(line.Side), int64ToString(line.AmountMinor), string(line.AssetCode))
	}
	return Fingerprint(parts...)
}

// int64ToString renders minor units canonically for fingerprinting.
func int64ToString(v int64) string { return strconv.FormatInt(v, 10) }

// decodePostingResult restores a replayed response; corrupt records fail
// loudly instead of fabricating a result.
func decodePostingResult(response []byte) (port.PostingResult, error) {
	var result port.PostingResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.PostingResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.PostingID.String() == "" {
		return port.PostingResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
