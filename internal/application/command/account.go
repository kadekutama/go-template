package command

import (
	"context"
	"strings"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// AccountServiceParams carries dependencies for AccountService.
type AccountServiceParams struct {
	UoW      port.UnitOfWork
	Accounts repository.AccountRepository
	Clock    port.Clock
	IDs      port.IDGenerator
	Authz    port.Authorizer
}

// AccountService executes account lifecycle commands. Each command uses
// optimistic concurrency via ExpectedVersion, validates aggregate state,
// writes through repository ports, and stages facts through the outbox
// store into the commit transaction (ambient binding documented in E06-T13).
type AccountService struct {
	uow      port.UnitOfWork
	accounts repository.AccountRepository
	clock    port.Clock
	ids      port.IDGenerator
	authz    port.Authorizer
}

// NewAccountService constructs an AccountService with the supplied dependencies.
func NewAccountService(params AccountServiceParams) *AccountService {
	return &AccountService{
		uow:      params.UoW,
		accounts: params.Accounts,
		clock:    params.Clock,
		ids:      params.IDs,
		authz:    params.Authz,
	}
}

var _ port.AccountCommandUseCases = (*AccountService)(nil)

// OpenAccount creates one ACTIVE account at version 1. Strong write.
func (s *AccountService) OpenAccount(ctx context.Context, req port.OpenAccountRequest) (port.AccountResult, error) {
	if err := validateOpenAccount(req); err != nil {
		return port.AccountResult{}, err
	}
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	return s.runAccountCommand(ctx, req.TenantID, req.Actor, "account.open", "ledger/"+string(req.LedgerID), req.IdempotencyKey,
		accountFingerprint(append([]string{"open", string(req.TenantID), string(req.LedgerID), req.IdempotencyKey, req.Number, req.Name, string(req.Class), string(req.AssetCode), req.Purpose}, MapParts("metadata", req.Metadata)...)...),
		func(ctx context.Context, tx port.Tx) (entity.AccountData, event.DomainEvent, error) {
			account, err := aggregate.OpenAccount(aggregate.OpenAccountParams{
				ID: "", TenantID: req.TenantID, LedgerID: req.LedgerID,
				Number: req.Number, Name: req.Name, Class: req.Class, AssetCode: req.AssetCode,
				Purpose: req.Purpose, Metadata: req.Metadata, OpenedBy: valueobject.UserID(req.Actor),
				EventID: eventID, OccurredAt: now,
			})
			if err != nil {
				return entity.AccountData{}, nil, err
			}
			stored, err := s.accounts.Create(ctx, account.Record())
			if err != nil {
				return entity.AccountData{}, nil, err
			}
			if err := account.AssignID(stored.ID, eventID, now, valueobject.UserID(req.Actor)); err != nil {
				return entity.AccountData{}, nil, err
			}
			return stored, firstEvent(account), nil
		})
}

// UpdateAccount mutates name/purpose/metadata under optimistic locking. Strong write.
func (s *AccountService) UpdateAccount(ctx context.Context, req port.UpdateAccountRequest) (port.AccountResult, error) {
	if err := validateAccountMutation(req.TenantID, req.AccountID, req.Actor, req.ExpectedVersion); err != nil {
		return port.AccountResult{}, err
	}
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	return s.runAccountCommand(ctx, req.TenantID, req.Actor, "account.update", "account/"+string(req.AccountID), updateAccountKey(req),
		accountFingerprint(append([]string{"update", string(req.TenantID), "", updateAccountKey(req), req.Name, req.Purpose}, MapParts("metadata", req.Metadata)...)...),
		func(ctx context.Context, tx port.Tx) (entity.AccountData, event.DomainEvent, error) {
			stored, err := s.accounts.FindByID(ctx, req.TenantID, req.AccountID)
			if err != nil {
				return entity.AccountData{}, nil, err
			}
			account := aggregate.LoadAccount(stored)
			if err := account.UpdateDetails(aggregate.UpdateDetailsParams{
				TransitionParams: aggregate.TransitionParams{Actor: valueobject.UserID(req.Actor), EventID: eventID, OccurredAt: now},
				Name:             req.Name,
				Purpose:          req.Purpose,
				Metadata:         req.Metadata,
			}); err != nil {
				return entity.AccountData{}, nil, err
			}
			record := account.Record()
			if err := s.accounts.UpdateMetadata(ctx, record, req.ExpectedVersion); err != nil {
				return entity.AccountData{}, nil, err
			}
			return record, firstEvent(account), nil
		})
}

// FreezeAccount moves ACTIVE→FROZEN. Strong write.
func (s *AccountService) FreezeAccount(ctx context.Context, req port.AccountLifecycleRequest) (port.AccountResult, error) {
	return s.transitionAccount(ctx, req, "account.freeze", func(account *aggregate.Account, params aggregate.TransitionParams, reason string) error {
		return account.Freeze(aggregate.FreezeParams{TransitionParams: params, Reason: reason})
	}, s.accounts.UpdateStatus)
}

// UnfreezeAccount moves FROZEN→ACTIVE. Strong write.
func (s *AccountService) UnfreezeAccount(ctx context.Context, req port.AccountLifecycleRequest) (port.AccountResult, error) {
	return s.transitionAccount(ctx, req, "account.unfreeze", func(account *aggregate.Account, params aggregate.TransitionParams, _ string) error {
		return account.Unfreeze(params)
	}, s.accounts.UpdateStatus)
}

// CloseAccount terminally closes an account. Strong write.
func (s *AccountService) CloseAccount(ctx context.Context, req port.AccountLifecycleRequest) (port.AccountResult, error) {
	return s.transitionAccount(ctx, req, "account.close", func(account *aggregate.Account, params aggregate.TransitionParams, reason string) error {
		return account.Close(aggregate.CloseParams{TransitionParams: params, Reason: reason})
	}, s.accounts.UpdateStatus)
}

// transitionAccount runs one lifecycle transition with optimistic locking:
// the aggregate owns legality, the repository owns the version check.
func (s *AccountService) transitionAccount(ctx context.Context, req port.AccountLifecycleRequest, action string,
	transition func(account *aggregate.Account, params aggregate.TransitionParams, reason string) error,
	persist func(ctx context.Context, tenant valueobject.TenantID, id valueobject.AccountID, status valueobject.AccountStatus, expectedVersion int64) error,
) (port.AccountResult, error) {
	if err := validateAccountMutation(req.TenantID, req.AccountID, req.Actor, req.ExpectedVersion); err != nil {
		return port.AccountResult{}, err
	}
	eventID := s.ids.NewID()
	now := s.clock.Now().UTC()
	return s.runAccountCommand(ctx, req.TenantID, req.Actor, action, "account/"+string(req.AccountID), lifecycleAccountKey(req, action),
		accountFingerprint(action, string(req.TenantID), "", lifecycleAccountKey(req, action), req.Reason),
		func(ctx context.Context, tx port.Tx) (entity.AccountData, event.DomainEvent, error) {
			stored, err := s.accounts.FindByID(ctx, req.TenantID, req.AccountID)
			if err != nil {
				return entity.AccountData{}, nil, err
			}
			account := aggregate.LoadAccount(stored)
			params := aggregate.TransitionParams{Actor: valueobject.UserID(req.Actor), EventID: eventID, OccurredAt: now}
			if err := transition(&account, params, req.Reason); err != nil {
				return entity.AccountData{}, nil, err
			}
			record := account.Record()
			if err := persist(ctx, req.TenantID, req.AccountID, record.Status, req.ExpectedVersion); err != nil {
				return entity.AccountData{}, nil, err
			}
			return record, firstEvent(account), nil
		})
}

// runAccountCommand authorizes, reserves idempotency inside one UnitOfWork,
// and either replays the stored response or executes the transition exactly
// once, staging the domain event as an outbox fact with the writes.
func (s *AccountService) runAccountCommand(ctx context.Context, tenant valueobject.TenantID, actor, action, resource, key, fingerprint string,
	execute func(ctx context.Context, tx port.Tx) (entity.AccountData, event.DomainEvent, error),
) (port.AccountResult, error) {
	subject := port.Subject{ID: actor, TenantID: tenant}
	if err := RequireAuthz(ctx, s.authz, subject, action, resource); err != nil {
		return port.AccountResult{}, err
	}
	rec := port.IdempotencyRecord{Key: key, Fingerprint: fingerprint, TenantID: tenant}
	var result port.AccountResult
	err := s.uow.Do(ctx, func(ctx context.Context, tx port.Tx) error {
		outcome, err := tx.Idempotency().Reserve(ctx, rec)
		if err != nil {
			return err
		}
		if outcome.Replay {
			decoded, err := decodeAccountResult(outcome.Response)
			if err != nil {
				return err
			}
			result = decoded
			return nil
		}
		record, evt, err := execute(ctx, tx)
		if err != nil {
			return err
		}
		result = port.AccountResult{Account: record, Cursor: tx.Cursor()}
		encoded, err := jsonparser.Marshal(result)
		if err != nil {
			return err
		}
		fact := port.OutboxFact{TenantID: tenant, OccurredAt: s.clock.Now().UTC(), Payload: encoded}
		if evt != nil {
			fact.EventType = evt.EventType()
			fact.AggregateID = evt.AggregateID()
			if payload, err := jsonparser.Marshal(evt.Payload()); err == nil {
				fact.Payload = payload
			}
		}
		if fact.LedgerID.String() == "" {
			fact.LedgerID = record.LedgerID
		}
		if err := tx.Outbox().Append(ctx, fact); err != nil {
			return err
		}
		return tx.Idempotency().Complete(ctx, rec.Key, encoded)
	})
	if err != nil {
		return port.AccountResult{}, err
	}
	return result, nil
}

// validateOpenAccount checks the open envelope; classification rules belong
// to the aggregate.
func validateOpenAccount(req port.OpenAccountRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(req.Actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "account actor is required")
	}
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		return entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "account command requires an idempotency key")
	}
	if strings.TrimSpace(req.Number) == "" {
		return entity.NewError("ACCOUNT_NUMBER_REQUIRED", "account number is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required")
	}
	return nil
}

// validateAccountMutation checks the mutation envelope.
func validateAccountMutation(tenant valueobject.TenantID, id valueobject.AccountID, actor string, expectedVersion int64) error {
	if strings.TrimSpace(tenant.String()) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(id.String()) == "" {
		return entity.NewError("ACCOUNT_ID_REQUIRED", "account id is required")
	}
	if strings.TrimSpace(actor) == "" {
		return entity.NewError("ACTOR_REQUIRED", "account actor is required")
	}
	if expectedVersion < 1 {
		return entity.NewError("ACCOUNT_VERSION_REQUIRED", "expected version must be at least 1")
	}
	return nil
}

// updateAccountKey derives a stable idempotency key scope for updates from
// the account + expected version.
func updateAccountKey(req port.UpdateAccountRequest) string {
	return "update:" + string(req.AccountID) + ":" + int64ToString(req.ExpectedVersion)
}

// lifecycleAccountKey scopes lifecycle idempotency by account + action +
// expected version so retries share the key while distinct intents do not.
func lifecycleAccountKey(req port.AccountLifecycleRequest, action string) string {
	return action + ":" + string(req.AccountID) + ":" + int64ToString(req.ExpectedVersion)
}

// accountFingerprint canonicalizes an account command for idempotency.
func accountFingerprint(parts ...string) string {
	return Fingerprint(parts...)
}

// firstEvent returns the first uncommitted domain event, if any.
func firstEvent(account aggregate.Account) event.DomainEvent {
	events := account.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}
	return events[0]
}

// decodeAccountResult restores a replayed response; corrupt records fail
// loudly instead of fabricating a result.
func decodeAccountResult(response []byte) (port.AccountResult, error) {
	var result port.AccountResult
	if err := jsonparser.Unmarshal(response, &result); err != nil {
		return port.AccountResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	if result.Account.ID.String() == "" {
		return port.AccountResult{}, entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt")
	}
	return result, nil
}
