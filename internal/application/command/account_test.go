package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

const (
	acctTenant = valueobject.TenantID("10000000-0000-4000-8000-000000000001")
	acctLedger = valueobject.LedgerID("20000000-0000-4000-8000-000000000001")
	acctID1    = valueobject.AccountID("40000000-0000-4000-8000-000000000001")
	acctAsset  = valueobject.AssetCode("USD")
)

var acctAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type accountTestFixture struct {
	uow    *mockapplication.MockUnitOfWork
	tx     *mockapplication.MockTx
	repo   *mockdomain.MockAccountRepository
	clock  *mockapplication.MockClock
	ids    *mockapplication.MockIDGenerator
	authz  *mockapplication.MockAuthorizer
	idem   *mockapplication.MockIdempotencyStore
	outbox *mockapplication.MockEventOutbox
	svc    *command.AccountService
}

func newAccountFixture(t *testing.T) *accountTestFixture {
	t.Helper()

	uow := mockapplication.NewMockUnitOfWork(t)
	tx := mockapplication.NewMockTx(t)
	repo := mockdomain.NewMockAccountRepository(t)
	clock := mockapplication.NewMockClock(t)
	ids := mockapplication.NewMockIDGenerator(t)
	authz := mockapplication.NewMockAuthorizer(t)
	idem := mockapplication.NewMockIdempotencyStore(t)
	outbox := mockapplication.NewMockEventOutbox(t)

	uow.EXPECT().Do(mock.Anything, mock.Anything).RunAndReturn(func(ctx context.Context, fn func(context.Context, port.Tx) error) error {
		return fn(ctx, tx)
	}).Maybe()

	tx.EXPECT().Idempotency().Return(idem).Maybe()
	tx.EXPECT().Outbox().Return(outbox).Maybe()
	tx.EXPECT().Cursor().Return("cursor-3").Maybe()

	clock.EXPECT().Now().Return(acctAt).Maybe()
	ids.EXPECT().NewID().Return(string(acctID1)).Maybe()

	idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Maybe()
	idem.EXPECT().Complete(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Maybe()

	svc := command.NewAccountService(command.AccountServiceParams{
		UoW:      uow,
		Accounts: repo,
		Clock:    clock,
		IDs:      ids,
		Authz:    authz,
	})

	return &accountTestFixture{
		uow:    uow,
		tx:     tx,
		repo:   repo,
		clock:  clock,
		ids:    ids,
		authz:  authz,
		idem:   idem,
		outbox: outbox,
		svc:    svc,
	}
}

func openTestAccount() port.OpenAccountRequest {
	return port.OpenAccountRequest{
		TenantID:       acctTenant,
		LedgerID:       acctLedger,
		Number:         "6000",
		Name:           "operating",
		Class:          valueobject.ClassLiability,
		AssetCode:      acctAsset,
		Purpose:        "ops",
		IdempotencyKey: "key-1",
		Actor:          "u-1",
	}
}

func TestAccountOpen(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		req            port.OpenAccountRequest
		setup          func(f *accountTestFixture)
		expectedStatus valueobject.AccountStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "open creates active record with event fact",
			req:  openTestAccount(),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.open", "ledger/"+string(acctLedger)).Return(nil)
				f.repo.EXPECT().Create(mock.Anything, mock.MatchedBy(func(data entity.AccountData) bool {
					return data.Number == "6000" && data.Name == "operating"
				})).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					LedgerID:  acctLedger,
					Number:    "6000",
					Name:      "operating",
					Class:     valueobject.ClassLiability,
					AssetCode: acctAsset,
					Purpose:   "ops",
					Status:    valueobject.StatusActive,
					Version:   1,
				}, nil)
			},
			expectedStatus: valueobject.StatusActive,
			expectedError:  nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  openTestAccount(),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.open", "ledger/"+string(acctLedger)).Return(nil)
				f.idem.ExpectedCalls = nil
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{
					Replay:   true,
					Response: []byte("{corrupt-json"),
				}, nil)
			},
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "duplicate replay returns original single record",
			req:  openTestAccount(),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.open", "ledger/"+string(acctLedger)).Return(nil)
				cachedResult := port.AccountResult{
					Account: entity.AccountData{ID: acctID1, Status: valueobject.StatusActive, Version: 1},
					Cursor:  "cursor-3",
				}
				cachedBytes, _ := jsonparser.Marshal(cachedResult)
				f.idem.ExpectedCalls = nil
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{
					Replay:   true,
					Response: cachedBytes,
				}, nil)
			},
			expectedStatus: valueobject.StatusActive,
			expectedError:  nil,
		},
		{
			name: "same key different name conflicts",
			req: func() port.OpenAccountRequest {
				r := openTestAccount()
				r.Name = "changed"
				return r
			}(),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.open", "ledger/"+string(acctLedger)).Return(nil)
				f.idem.ExpectedCalls = nil
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"))
			},
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key leased for a different request"),
		},
		{
			name: "missing number fails validation",
			req: func() port.OpenAccountRequest {
				r := openTestAccount()
				r.Number = ""
				return r
			}(),
			setup:          func(_ *accountTestFixture) {},
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_NUMBER_REQUIRED", "account number is required"),
		},
		{
			name: "denied subject fails before store touch",
			req:  openTestAccount(),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.open", "ledger/"+string(acctLedger)).Return(entity.NewError("FORBIDDEN", "subject is not authorized for this action"))
			},
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountFixture(t)
			tc.setup(f)

			actualResult, err := f.svc.OpenAccount(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Account.Status)
				assert.Equal(t, int64(1), actualResult.Account.Version)
				assert.Equal(t, "cursor-3", actualResult.Cursor)
			}
		})
	}
}

func TestAccountLifecycle(t *testing.T) {
	t.Parallel()

	lifecycleReq := func(id valueobject.AccountID, version int64) port.AccountLifecycleRequest {
		return port.AccountLifecycleRequest{
			TenantID:        acctTenant,
			AccountID:       id,
			ExpectedVersion: version,
			Reason:          "review",
			Actor:           "u-1",
		}
	}

	type testCase struct {
		name           string
		action         string
		req            port.AccountLifecycleRequest
		setup          func(f *accountTestFixture)
		expectedStatus valueobject.AccountStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:   "freeze active succeeds",
			action: "freeze",
			req:    lifecycleReq(acctID1, 1),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.freeze", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Status:    valueobject.StatusActive,
					Version:   1,
					AssetCode: acctAsset,
				}, nil)
				f.repo.EXPECT().UpdateStatus(mock.Anything, acctTenant, acctID1, valueobject.StatusFrozen, int64(1)).Return(nil)
			},
			expectedStatus: valueobject.StatusFrozen,
			expectedError:  nil,
		},
		{
			name:   "freeze active then double freeze fails",
			action: "freeze",
			req:    lifecycleReq(acctID1, 2),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.freeze", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Status:    valueobject.StatusFrozen,
					Version:   2,
					AssetCode: acctAsset,
				}, nil)
			},
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_ALREADY_FROZEN", "account is already frozen"),
		},
		{
			name:   "unfreeze non-frozen fails",
			action: "unfreeze",
			req:    lifecycleReq(acctID1, 1),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.unfreeze", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Status:    valueobject.StatusActive,
					Version:   1,
					AssetCode: acctAsset,
				}, nil)
			},
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_NOT_FROZEN", "account is not frozen"),
		},
		{
			name:   "close frozen succeeds terminally",
			action: "close",
			req:    lifecycleReq(acctID1, 2),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.close", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Status:    valueobject.StatusFrozen,
					Version:   2,
					AssetCode: acctAsset,
				}, nil)
				f.repo.EXPECT().UpdateStatus(mock.Anything, acctTenant, acctID1, valueobject.StatusClosed, int64(2)).Return(nil)
			},
			expectedStatus: valueobject.StatusClosed,
			expectedError:  nil,
		},
		{
			name:   "stale version conflicts without write",
			action: "freeze",
			req:    lifecycleReq(acctID1, 99),
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.freeze", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Status:    valueobject.StatusActive,
					Version:   1,
					AssetCode: acctAsset,
				}, nil)
				f.repo.EXPECT().UpdateStatus(mock.Anything, acctTenant, acctID1, valueobject.StatusFrozen, int64(99)).Return(
					entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
				)
			},
			expectedStatus: "",
			expectedError:  entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountFixture(t)
			tc.setup(f)

			var actualResult port.AccountResult
			var err error
			switch tc.action {
			case "freeze":
				actualResult, err = f.svc.FreezeAccount(context.Background(), tc.req)
			case "unfreeze":
				actualResult, err = f.svc.UnfreezeAccount(context.Background(), tc.req)
			case "close":
				actualResult, err = f.svc.CloseAccount(context.Background(), tc.req)
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
		AccountID:       acctID1,
		Name:            "Operating Account Updated",
		Purpose:         "Updated treasury ops",
		Metadata:        map[string]string{"env": "staging"},
		ExpectedVersion: 1,
		Actor:           "u-1",
	}

	type testCase struct {
		name           string
		req            port.UpdateAccountRequest
		setup          func(f *accountTestFixture)
		expectedResult port.AccountResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "successful update bumps version and mutates details",
			req:  baseReq,
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.update", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					LedgerID:  acctLedger,
					Number:    "1000",
					Name:      "Operating Account",
					Class:     valueobject.ClassLiability,
					AssetCode: acctAsset,
					Purpose:   "ops",
					Status:    valueobject.StatusActive,
					Version:   1,
					CreatedAt: acctAt,
					UpdatedAt: acctAt,
				}, nil)
				f.repo.EXPECT().UpdateMetadata(mock.Anything, mock.MatchedBy(func(data entity.AccountData) bool {
					return data.Name == "Operating Account Updated" && data.Purpose == "Updated treasury ops"
				}), int64(1)).Return(nil)
			},
			expectedResult: port.AccountResult{
				Account: entity.AccountData{
					ID:        acctID1,
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
				Cursor: "cursor-3",
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
			setup:          func(_ *accountTestFixture) {},
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
			setup:          func(_ *accountTestFixture) {},
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
			setup:          func(_ *accountTestFixture) {},
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
			setup:          func(_ *accountTestFixture) {},
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
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.update", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Version:   1,
					AssetCode: acctAsset,
				}, nil)
			},
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required"),
		},
		{
			name: "account not found returns error",
			req:  baseReq,
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.update", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{}, entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"))
			},
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
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.update", "account/"+string(acctID1)).Return(nil)
				f.repo.EXPECT().FindByID(mock.Anything, acctTenant, acctID1).Return(entity.AccountData{
					ID:        acctID1,
					TenantID:  acctTenant,
					Version:   1,
					AssetCode: acctAsset,
				}, nil)
				f.repo.EXPECT().UpdateMetadata(mock.Anything, mock.Anything, int64(99)).Return(
					entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
				)
			},
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
		{
			name: "denied subject returns forbidden error",
			req:  baseReq,
			setup: func(f *accountTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "account.update", "account/"+string(acctID1)).Return(entity.NewError("FORBIDDEN", "subject is not authorized for this action"))
			},
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := newAccountFixture(t)
			tc.setup(f)

			result, err := f.svc.UpdateAccount(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedResult.Account.Name, result.Account.Name)
				assert.Equal(t, tc.expectedResult.Account.Purpose, result.Account.Purpose)
				assert.Equal(t, tc.expectedResult.Account.Version, result.Account.Version)
				assert.Equal(t, tc.expectedResult.Account.Metadata, result.Account.Metadata)
			}
		})
	}
}
