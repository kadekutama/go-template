package command_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	acctTenant = valueobject.TenantID("t-1")
	acctLedger = valueobject.LedgerID("l-1")
	acctAsset  = valueobject.AssetCode("USD")
)

var acctAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type acctRepo struct {
	mu       sync.Mutex
	accounts map[valueobject.AccountID]entity.AccountData
}

func (f *acctRepo) Create(_ context.Context, account entity.AccountData) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, dup := f.accounts[account.ID]; dup {
		return entity.NewError("ACCOUNT_CONFLICT", "account id already exists")
	}
	if f.accounts == nil {
		f.accounts = map[valueobject.AccountID]entity.AccountData{}
	}
	f.accounts[account.ID] = account
	return nil
}

func (f *acctRepo) FindByID(_ context.Context, _ valueobject.TenantID, id valueobject.AccountID) (entity.AccountData, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	account, ok := f.accounts[id]
	if !ok {
		return entity.AccountData{}, entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown")
	}
	return account, nil
}

func (f *acctRepo) FindByTenant(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]entity.AccountData, string, error) {
	return nil, "", nil
}

func (f *acctRepo) UpdateMetadata(_ context.Context, account entity.AccountData, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := f.accounts[account.ID]
	if !ok {
		return entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown")
	}
	if stored.Version != expectedVersion {
		return entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry")
	}
	f.accounts[account.ID] = account
	return nil
}

func (f *acctRepo) UpdateStatus(_ context.Context, _ valueobject.TenantID, id valueobject.AccountID, status valueobject.AccountStatus, expectedVersion int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	stored, ok := f.accounts[id]
	if !ok {
		return entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown")
	}
	if stored.Version != expectedVersion {
		return entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry")
	}
	stored.Status = status
	stored.Version++
	f.accounts[id] = stored
	return nil
}

type acctIdemEntry struct {
	fingerprint string
	response    []byte
	completed   bool
}

type acctUOW struct {
	mu     sync.Mutex
	outbox []port.OutboxFact
	idem   map[string]acctIdemEntry
}

func (u *acctUOW) Do(ctx context.Context, fn func(ctx context.Context, tx port.Tx) error) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	stagedOutbox := []port.OutboxFact{}
	stagedIdem := map[string]acctIdemEntry{}
	tx := &acctTx{uow: u, outbox: &stagedOutbox, idem: stagedIdem}
	if err := fn(ctx, tx); err != nil {
		return err
	}
	u.outbox = append(u.outbox, stagedOutbox...)
	if u.idem == nil {
		u.idem = map[string]acctIdemEntry{}
	}
	for key, entry := range stagedIdem {
		u.idem[key] = entry
	}
	return nil
}

type acctTx struct {
	uow    *acctUOW
	outbox *[]port.OutboxFact
	idem   map[string]acctIdemEntry
}

func (t *acctTx) Postings() repository.PostingRepository { return nil }
func (t *acctTx) Holds() repository.HoldRepository       { return nil }
func (t *acctTx) Idempotency() port.IdempotencyStore     { return &acctTxIdem{tx: t} }
func (t *acctTx) Outbox() port.EventOutbox               { return &acctTxOutbox{tx: t} }
func (t *acctTx) Cursor() string                         { return "cursor-3" }

type acctTxIdem struct {
	tx *acctTx
}

func (s *acctTxIdem) Reserve(_ context.Context, rec port.IdempotencyRecord) (port.ReserveOutcome, error) {
	if entry, ok := s.tx.uow.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		if entry.completed {
			return port.ReserveOutcome{Replay: true, Response: entry.response}, nil
		}
		return port.ReserveOutcome{}, nil
	}
	if entry, ok := s.tx.idem[rec.Key]; ok {
		if entry.fingerprint != rec.Fingerprint {
			return port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request")
		}
		return port.ReserveOutcome{}, nil
	}
	s.tx.idem[rec.Key] = acctIdemEntry{fingerprint: rec.Fingerprint}
	return port.ReserveOutcome{}, nil
}

func (s *acctTxIdem) Complete(_ context.Context, key string, response []byte) error {
	entry := s.tx.idem[key]
	entry.response = response
	entry.completed = true
	s.tx.idem[key] = entry
	return nil
}

type acctTxOutbox struct {
	tx *acctTx
}

func (o *acctTxOutbox) Append(_ context.Context, facts ...port.OutboxFact) error {
	*o.tx.outbox = append(*o.tx.outbox, facts...)
	return nil
}

type acctClock struct{}

func (acctClock) Now() time.Time { return acctAt }

type acctIDs struct {
	next []string
}

func (f *acctIDs) NewID() string {
	id := f.next[0]
	f.next = f.next[1:]
	return id
}

func acctTestIDs() []string {
	ids := make([]string, 0, 40)
	for i := 1; i <= 40; i++ {
		ids = append(ids, fmt.Sprintf("acct-%02d", i))
	}
	return ids
}

type acctAuthz struct {
	denied map[string]bool
	calls  int
}

func (a *acctAuthz) Authorize(_ context.Context, subject port.Subject, action, resource string) error {
	a.calls++
	if a.denied[subject.ID+"|"+action+"|"+resource] {
		return entity.NewError("FORBIDDEN", "subject is not authorized for this action")
	}
	return nil
}

func newAccountService(uow *acctUOW, repo *acctRepo, authz *acctAuthz) *command.AccountService {
	return command.NewAccountService(command.AccountServiceParams{
		UoW:      uow,
		Accounts: repo,
		Clock:    acctClock{},
		IDs:      &acctIDs{next: acctTestIDs()},
		Authz:    authz,
	})
}

func openTestAccount() port.OpenAccountRequest {
	return port.OpenAccountRequest{
		TenantID: acctTenant, LedgerID: acctLedger, Number: "6000", Name: "operating",
		Class: valueobject.ClassLiability, AssetCode: acctAsset, Purpose: "ops",
		IdempotencyKey: "key-1", Actor: "u-1",
	}
}

func TestAccountOpen(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		req                port.OpenAccountRequest
		preload            func(uow *acctUOW, svc *command.AccountService)
		denied             bool
		expectedStatus     valueobject.AccountStatus
		expectedError      error
		expectedRecords    int
		expectedOutbox     int
		expectedAuthzCalls int
	}

	testCases := []testCase{
		{
			name:               "open creates active record with event fact",
			req:                openTestAccount(),
			preload:            func(_ *acctUOW, _ *command.AccountService) {},
			denied:             false,
			expectedStatus:     valueobject.StatusActive,
			expectedError:      nil,
			expectedRecords:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 1,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  openTestAccount(),
			preload: func(uow *acctUOW, _ *command.AccountService) {
				req := openTestAccount()
				fp := command.Fingerprint(append([]string{
					"open", string(req.TenantID), string(req.LedgerID), req.IdempotencyKey,
					req.Number, req.Name, string(req.Class), string(req.AssetCode), req.Purpose,
				}, command.MapParts("metadata", req.Metadata)...)...)
				uow.idem = map[string]acctIdemEntry{
					req.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			denied:             false,
			expectedStatus:     "",
			expectedError:      entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedRecords:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "duplicate replay returns original single record",
			req:  openTestAccount(),
			preload: func(_ *acctUOW, svc *command.AccountService) {
				_, err := svc.OpenAccount(context.Background(), openTestAccount())
				require.NoError(t, err)
			},
			denied:             false,
			expectedStatus:     valueobject.StatusActive,
			expectedError:      nil,
			expectedRecords:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
		},
		{
			name: "same key different name conflicts",
			req: func() port.OpenAccountRequest {
				r := openTestAccount()
				r.Name = "changed"
				return r
			}(),
			preload: func(_ *acctUOW, svc *command.AccountService) {
				_, err := svc.OpenAccount(context.Background(), openTestAccount())
				require.NoError(t, err)
			},
			denied:             false,
			expectedStatus:     "",
			expectedError:      entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
			expectedRecords:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
		},
		{
			name: "missing number fails validation",
			req: func() port.OpenAccountRequest {
				r := openTestAccount()
				r.Number = ""
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.AccountService) {},
			denied:             false,
			expectedStatus:     "",
			expectedError:      entity.NewError("ACCOUNT_NUMBER_REQUIRED", "account number is required"),
			expectedRecords:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
		{
			name:               "denied subject fails before store touch",
			req:                openTestAccount(),
			preload:            func(_ *acctUOW, _ *command.AccountService) {},
			denied:             true,
			expectedStatus:     "",
			expectedError:      entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedRecords:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &acctUOW{}
			repo := &acctRepo{}
			authz := &acctAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied["u-1|account.open|ledger/l-1"] = true
			}
			svc := newAccountService(uow, repo, authz)
			tc.preload(uow, svc)
			actualResult, err := svc.OpenAccount(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Account.Status)
				assert.Equal(t, int64(1), actualResult.Account.Version)
				assert.Equal(t, "cursor-3", actualResult.Cursor)
			}
			assert.Len(t, repo.accounts, tc.expectedRecords)
			assert.Len(t, uow.outbox, tc.expectedOutbox)
			assert.Equal(t, tc.expectedAuthzCalls, authz.calls)
		})
	}
}

func TestAccountLifecycle(t *testing.T) {
	t.Parallel()

	seed := func(t *testing.T, svc *command.AccountService) port.AccountResult {
		t.Helper()
		res, err := svc.OpenAccount(context.Background(), openTestAccount())
		require.NoError(t, err)
		return res
	}

	lifecycleReq := func(id valueobject.AccountID, version int64) port.AccountLifecycleRequest {
		return port.AccountLifecycleRequest{
			TenantID: acctTenant, AccountID: id, ExpectedVersion: version,
			Reason: "review", Actor: "u-1",
		}
	}

	type testCase struct {
		name           string
		prepare        func(svc *command.AccountService) port.AccountResult
		action         string
		version        int64
		expectedStatus valueobject.AccountStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "freeze active then double freeze fails",
			prepare: func(svc *command.AccountService) port.AccountResult {
				opened := seed(t, svc)
				frozen, err := svc.FreezeAccount(context.Background(), lifecycleReq(opened.Account.ID, opened.Account.Version))
				require.NoError(t, err)
				require.Equal(t, valueobject.StatusFrozen, frozen.Account.Status)
				return opened
			},
			action:         "freeze",
			version:        2,
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_ALREADY_FROZEN", "account is already frozen"),
		},
		{
			name: "unfreeze non-frozen fails",
			prepare: func(svc *command.AccountService) port.AccountResult {
				return seed(t, svc)
			},
			action:         "unfreeze",
			version:        1,
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_NOT_FROZEN", "account is not frozen"),
		},
		{
			name: "close frozen succeeds terminally",
			prepare: func(svc *command.AccountService) port.AccountResult {
				opened := seed(t, svc)
				_, err := svc.FreezeAccount(context.Background(), lifecycleReq(opened.Account.ID, opened.Account.Version))
				require.NoError(t, err)
				return opened
			},
			action:         "close",
			version:        2,
			expectedStatus: valueobject.StatusClosed,
			expectedError:  nil,
		},
		{
			name: "stale version conflicts without write",
			prepare: func(svc *command.AccountService) port.AccountResult {
				return seed(t, svc)
			},
			action:         "freeze",
			version:        99,
			expectedStatus: "",
			expectedError:  entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &acctUOW{}
			repo := &acctRepo{}
			authz := &acctAuthz{denied: map[string]bool{}}
			svc := newAccountService(uow, repo, authz)
			opened := tc.prepare(svc)
			req := lifecycleReq(opened.Account.ID, tc.version)
			var actualResult port.AccountResult
			var err error
			switch tc.action {
			case "freeze":
				actualResult, err = svc.FreezeAccount(context.Background(), req)
			case "unfreeze":
				actualResult, err = svc.UnfreezeAccount(context.Background(), req)
			case "close":
				actualResult, err = svc.CloseAccount(context.Background(), req)
			}
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Account.Status)
			}
		})
	}
}

func TestUpdateAccount(t *testing.T) {
	t.Parallel()

	baseReq := port.UpdateAccountRequest{
		TenantID:        acctTenant,
		AccountID:       valueobject.AccountID("acct-1"),
		Name:            "Operating Account Updated",
		Purpose:         "Updated treasury ops",
		Metadata:        map[string]string{"env": "staging"},
		ExpectedVersion: 1,
		Actor:           "u-1",
	}

	type testCase struct {
		name           string
		req            port.UpdateAccountRequest
		seedAccount    bool
		denied         bool
		expectedResult port.AccountResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name:        "successful update bumps version and mutates details",
			req:         baseReq,
			seedAccount: true,
			denied:      false,
			expectedResult: port.AccountResult{
				Account: entity.AccountData{
					ID:        valueobject.AccountID("acct-1"),
					TenantID:  acctTenant,
					LedgerID:  acctLedger,
					Number:    "1000",
					Name:      "Operating Account Updated",
					Class:     valueobject.ClassLiability,
					AssetCode: acctAsset,
					Purpose:   "Updated treasury ops",
					Status:    valueobject.StatusActive,
					Version:   2,
					Metadata:  map[string]string{"env": "staging"},
					CreatedAt: acctAt,
					UpdatedAt: acctAt,
				},
				Cursor: "1",
			},
			expectedError: nil,
		},
		{
			name: "missing tenant rejected",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			seedAccount:    false,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing account id rejected",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.AccountID = ""
				return r
			}(),
			seedAccount:    false,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_ID_REQUIRED", "account id is required"),
		},
		{
			name: "missing actor rejected",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			seedAccount:    false,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACTOR_REQUIRED", "account actor is required"),
		},
		{
			name: "zero expected version rejected",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.ExpectedVersion = 0
				return r
			}(),
			seedAccount:    false,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_VERSION_REQUIRED", "expected version must be at least 1"),
		},
		{
			name: "empty account name rejected",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.Name = ""
				return r
			}(),
			seedAccount:    true,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required"),
		},
		{
			name:           "account not found returns error",
			req:            baseReq,
			seedAccount:    false,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
		},
		{
			name: "version conflict on stale version",
			req: func() port.UpdateAccountRequest {
				r := baseReq
				r.ExpectedVersion = 99
				return r
			}(),
			seedAccount:    true,
			denied:         false,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
		{
			name:           "denied subject returns forbidden error",
			req:            baseReq,
			seedAccount:    true,
			denied:         true,
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &acctUOW{}
			repo := &acctRepo{}
			authz := &acctAuthz{denied: map[string]bool{}}
			svc := newAccountService(uow, repo, authz)
			req := tc.req
			if tc.seedAccount {
				opened, err := svc.OpenAccount(context.Background(), openTestAccount())
				require.NoError(t, err)
				if req.AccountID == valueobject.AccountID("acct-1") {
					req.AccountID = opened.Account.ID
				}
			}
			if tc.denied {
				authz.denied[req.Actor+"|account.update|account/"+string(req.AccountID)] = true
			}

			result, err := svc.UpdateAccount(context.Background(), req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedResult.Account.Name, result.Account.Name)
				assert.Equal(t, tc.expectedResult.Account.Purpose, result.Account.Purpose)
				assert.Equal(t, tc.expectedResult.Account.Version, result.Account.Version)
				assert.Equal(t, tc.expectedResult.Account.Metadata, result.Account.Metadata)

				// Idempotency replay verification
				replayResult, replayErr := svc.UpdateAccount(context.Background(), req)
				assert.NoError(t, replayErr)
				assert.Equal(t, result.Account.ID, replayResult.Account.ID)
			}
		})
	}
}
