package command_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
)

var tenantFixedTime = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

type tenantTestFixture struct {
	uow     *mockapplication.MockUnitOfWork
	tx      *mockapplication.MockTx
	store   *mockcommand.MockTenantStore
	clock   *mockapplication.MockClock
	ids     *mockapplication.MockIDGenerator
	authz   *mockapplication.MockAuthorizer
	idem    *mockapplication.MockIdempotencyStore
	outbox  *mockapplication.MockEventOutbox
	service *command.TenantService
}

func newTenantFixture(t *testing.T) *tenantTestFixture {
	t.Helper()

	uow := mockapplication.NewMockUnitOfWork(t)
	tx := mockapplication.NewMockTx(t)
	store := mockcommand.NewMockTenantStore(t)
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

	clock.EXPECT().Now().Return(tenantFixedTime).Maybe()
	ids.EXPECT().NewID().Return("event-1").Maybe()

	svc, err := command.NewTenantService(command.TenantServiceParams{
		UoW:              uow,
		Tenants:          store,
		Clock:            clock,
		IDs:              ids,
		Authz:            authz,
		Regions:          []string{"us-east", "eu-west"},
		DefaultAssets:    []valueobject.AssetCode{"USD"},
		MaxAssets:        8,
		PlatformTenantID: valueobject.TenantID("00000000-0000-0000-0000-000000000000"),
	})
	require.NoError(t, err)

	return &tenantTestFixture{
		uow:     uow,
		tx:      tx,
		store:   store,
		clock:   clock,
		ids:     ids,
		authz:   authz,
		idem:    idem,
		outbox:  outbox,
		service: svc,
	}
}

func tenantSettings() entity.TenantSettings {
	return entity.TenantSettings{
		DefaultCurrency:       "USD",
		Timezone:              "UTC",
		EnabledFeatures:       []string{"transfers"},
		EnabledPaymentMethods: []string{"ACH"},
	}
}

func provisionTestTenant() port.ProvisionTenantRequest {
	return port.ProvisionTenantRequest{
		Name:           "acme",
		Region:         "us-east",
		Settings:       tenantSettings(),
		IdempotencyKey: "key-1",
		Actor:          "u-1",
	}
}

func TestTenantProvision(t *testing.T) {
	t.Parallel()

	baseReq := provisionTestTenant()

	type testCase struct {
		name           string
		req            port.ProvisionTenantRequest
		setupMocks     func(f *tenantTestFixture)
		expectedResult port.TenantResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "provision creates tenant with defaults",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().ListNames(mock.Anything).Return([]string{}, nil).Once()
				f.store.EXPECT().ListAliases(mock.Anything).Return([]string{}, nil).Once()
				f.store.EXPECT().Create(mock.Anything, mock.MatchedBy(func(td entity.TenantData) bool {
					return td.Name == "acme" && td.Alias == "acme" && td.Region == "us-east"
				})).Return(entity.TenantData{
					ID:        "t-100",
					Name:      "acme",
					Alias:     "acme",
					Status:    entity.TenantActive,
					Region:    "us-east",
					Settings:  tenantSettings(),
					CreatedAt: tenantFixedTime,
					UpdatedAt: tenantFixedTime,
					Version:   1,
				}, nil).Once()
				f.outbox.EXPECT().Append(mock.Anything, mock.MatchedBy(func(fact port.OutboxFact) bool {
					return fact.TenantID == "t-100" && fact.EventType == "tenant.created.v1"
				})).Return(nil).Once()
				f.idem.EXPECT().Complete(mock.Anything, "key-1", mock.Anything).Return(nil).Once()
			},
			expectedResult: port.TenantResult{
				Tenant: entity.TenantData{
					ID:        "t-100",
					Name:      "acme",
					Alias:     "acme",
					Status:    entity.TenantActive,
					Region:    "us-east",
					Settings:  tenantSettings(),
					CreatedAt: tenantFixedTime,
					UpdatedAt: tenantFixedTime,
					Version:   1,
				},
				Cursor: "cursor-3",
			},
			expectedError: nil,
		},
		{
			name: "duplicate replay returns original single tenant",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				replayResult := port.TenantResult{
					Tenant: entity.TenantData{
						ID:        "t-100",
						Name:      "acme",
						Alias:     "acme",
						Status:    entity.TenantActive,
						Region:    "us-east",
						Settings:  tenantSettings(),
						CreatedAt: tenantFixedTime,
						UpdatedAt: tenantFixedTime,
						Version:   1,
					},
					Cursor: "cursor-3",
				}
				payload, err := jsonparser.Marshal(replayResult)
				require.NoError(t, err)
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{
					Replay:   true,
					Response: payload,
				}, nil).Once()
			},
			expectedResult: port.TenantResult{
				Tenant: entity.TenantData{
					ID:        "t-100",
					Name:      "acme",
					Alias:     "acme",
					Status:    entity.TenantActive,
					Region:    "us-east",
					Settings:  tenantSettings(),
					CreatedAt: tenantFixedTime,
					UpdatedAt: tenantFixedTime,
					Version:   1,
				},
				Cursor: "cursor-3",
			},
			expectedError: nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{
					Replay:   true,
					Response: []byte("{corrupt-json"),
				}, nil).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "taken name fails without persisting",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.IdempotencyKey = "key-2"
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().ListNames(mock.Anything).Return([]string{"acme"}, nil).Once()
				f.store.EXPECT().ListAliases(mock.Anything).Return([]string{}, nil).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_NAME_DUPLICATE", "tenant name is already taken"),
		},
		{
			name: "disallowed region fails without persisting",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.Region = "antarctica"
				r.Name = "other"
				r.IdempotencyKey = "key-2"
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().ListNames(mock.Anything).Return([]string{}, nil).Once()
				f.store.EXPECT().ListAliases(mock.Anything).Return([]string{}, nil).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_REGION_INVALID", "tenant region is not allowed"),
		},
		{
			name: "missing name fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.Name = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
		{
			name: "missing region fails domain validation",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.Region = ""
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().ListNames(mock.Anything).Return([]string{}, nil).Once()
				f.store.EXPECT().ListAliases(mock.Anything).Return([]string{}, nil).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_REGION_REQUIRED", "tenant region is required"),
		},
		{
			name: "missing actor fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("ACTOR_REQUIRED", "tenant actor is required"),
		},
		{
			name: "missing idempotency key fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(
					entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
				).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
		{
			name: "invalid alias returns TENANT_ALIAS_INVALID",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.Alias = "INVALID_ALIAS!"
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_ALIAS_INVALID", "tenant alias must be lowercase alphanumeric with hyphens"),
		},
		{
			name: "taken alias resolves with suffix",
			req: func() port.ProvisionTenantRequest {
				r := baseReq
				r.IdempotencyKey = "key-2"
				r.Name = "Acme."
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.provision", "platform/tenants").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().ListNames(mock.Anything).Return([]string{}, nil).Once()
				f.store.EXPECT().ListAliases(mock.Anything).Return([]string{"acme"}, nil).Once()
				f.store.EXPECT().Create(mock.Anything, mock.MatchedBy(func(td entity.TenantData) bool {
					return td.Alias == "acme-2"
				})).Return(entity.TenantData{
					ID:        "t-101",
					Name:      "Acme.",
					Alias:     "acme-2",
					Status:    entity.TenantActive,
					Region:    "us-east",
					Settings:  tenantSettings(),
					CreatedAt: tenantFixedTime,
					UpdatedAt: tenantFixedTime,
					Version:   1,
				}, nil).Once()
				f.outbox.EXPECT().Append(mock.Anything, mock.Anything).Return(nil).Once()
				f.idem.EXPECT().Complete(mock.Anything, "key-2", mock.Anything).Return(nil).Once()
			},
			expectedResult: port.TenantResult{
				Tenant: entity.TenantData{
					ID:        "t-101",
					Name:      "Acme.",
					Alias:     "acme-2",
					Status:    entity.TenantActive,
					Region:    "us-east",
					Settings:  tenantSettings(),
					CreatedAt: tenantFixedTime,
					UpdatedAt: tenantFixedTime,
					Version:   1,
				},
				Cursor: "cursor-3",
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newTenantFixture(t)
			tc.setupMocks(fixture)
			actualResult, err := fixture.service.ProvisionTenant(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestTenantSettingsUpdate(t *testing.T) {
	t.Parallel()

	targetTenantID := valueobject.TenantID("10000000-0000-4000-8000-000000000001")
	baseTenant := entity.TenantData{
		ID:        targetTenantID,
		Name:      "acme",
		Alias:     "acme",
		Status:    entity.TenantActive,
		Region:    "us-east",
		Settings:  tenantSettings(),
		CreatedAt: tenantFixedTime,
		UpdatedAt: tenantFixedTime,
		Version:   1,
	}

	baseReq := port.UpdateTenantSettingsRequest{
		TenantID:        targetTenantID,
		Settings:        tenantSettings(),
		ExpectedVersion: 1,
		IdempotencyKey:  "key-settings-1",
		Actor:           "u-1",
	}

	type testCase struct {
		name           string
		req            port.UpdateTenantSettingsRequest
		setupMocks     func(f *tenantTestFixture)
		expectedResult port.TenantResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "current version updates settings",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.update", "tenant/"+targetTenantID.String()).Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().FindByID(mock.Anything, targetTenantID).Return(baseTenant, nil).Once()
				f.store.EXPECT().UpdateSettings(mock.Anything, mock.MatchedBy(func(td entity.TenantData) bool {
					return td.ID == targetTenantID
				}), int64(1)).Return(nil).Once()
				f.idem.EXPECT().Complete(mock.Anything, "key-settings-1", mock.Anything).Return(nil).Once()
			},
			expectedResult: port.TenantResult{
				Tenant: baseTenant,
				Cursor: "cursor-3",
			},
			expectedError: nil,
		},
		{
			name: "replay returns original settings without re-mutating",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.update", "tenant/"+targetTenantID.String()).Return(nil).Once()
				replayResult := port.TenantResult{
					Tenant: baseTenant,
					Cursor: "cursor-3",
				}
				payload, err := jsonparser.Marshal(replayResult)
				require.NoError(t, err)
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{
					Replay:   true,
					Response: payload,
				}, nil).Once()
			},
			expectedResult: port.TenantResult{
				Tenant: baseTenant,
				Cursor: "cursor-3",
			},
			expectedError: nil,
		},
		{
			name: "stale version conflicts without write",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.ExpectedVersion = 99
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.update", "tenant/"+targetTenantID.String()).Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().FindByID(mock.Anything, targetTenantID).Return(baseTenant, nil).Once()
				f.store.EXPECT().UpdateSettings(mock.Anything, mock.Anything, int64(99)).Return(
					entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
				).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
		{
			name: "nonexistent tenant fails",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.TenantID = "20000000-0000-4000-8000-000000000002"
				return r
			}(),
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.update", "tenant/20000000-0000-4000-8000-000000000002").Return(nil).Once()
				f.idem.EXPECT().Reserve(mock.Anything, mock.Anything).Return(port.ReserveOutcome{}, nil).Once()
				f.store.EXPECT().FindByID(mock.Anything, valueobject.TenantID("20000000-0000-4000-8000-000000000002")).Return(
					entity.TenantData{},
					entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
				).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
		},
		{
			name: "missing tenant ID fails envelope validation",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing actor fails envelope validation",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("ACTOR_REQUIRED", "tenant actor is required"),
		},
		{
			name: "zero expected version fails envelope validation",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.ExpectedVersion = 0
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_VERSION_INVALID", "expected version must be at least 1"),
		},
		{
			name: "missing idempotency key fails envelope validation",
			req: func() port.UpdateTenantSettingsRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			setupMocks:     func(_ *tenantTestFixture) {},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			req:  baseReq,
			setupMocks: func(f *tenantTestFixture) {
				f.authz.EXPECT().Authorize(mock.Anything, mock.Anything, "tenant.update", "tenant/"+targetTenantID.String()).Return(
					entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
				).Once()
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newTenantFixture(t)
			tc.setupMocks(fixture)
			actualResult, err := fixture.service.UpdateTenantSettings(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestNewTenantService(t *testing.T) {
	t.Parallel()

	uow := mockapplication.NewMockUnitOfWork(t)
	store := mockcommand.NewMockTenantStore(t)
	clock := mockapplication.NewMockClock(t)
	ids := mockapplication.NewMockIDGenerator(t)
	authz := mockapplication.NewMockAuthorizer(t)

	baseParams := command.TenantServiceParams{
		UoW:              uow,
		Tenants:          store,
		Clock:            clock,
		IDs:              ids,
		Authz:            authz,
		Regions:          []string{"us-east", "eu-west"},
		DefaultAssets:    []valueobject.AssetCode{"USD"},
		MaxAssets:        8,
		PlatformTenantID: valueobject.TenantID("00000000-0000-0000-0000-000000000000"),
	}

	type testCase struct {
		name          string
		params        command.TenantServiceParams
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid parameters",
			params:        baseParams,
			expectedError: nil,
		},
		{
			name: "missing uow rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.UoW = nil
				return p
			}(),
			expectedError: errors.New("tenant: uow is required"),
		},
		{
			name: "missing tenant store rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.Tenants = nil
				return p
			}(),
			expectedError: errors.New("tenant: tenant store is required"),
		},
		{
			name: "missing clock rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.Clock = nil
				return p
			}(),
			expectedError: errors.New("tenant: clock is required"),
		},
		{
			name: "missing ids rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.IDs = nil
				return p
			}(),
			expectedError: errors.New("tenant: id generator is required"),
		},
		{
			name: "missing authz rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.Authz = nil
				return p
			}(),
			expectedError: errors.New("tenant: authorizer is required"),
		},
		{
			name: "empty regions rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.Regions = nil
				return p
			}(),
			expectedError: errors.New("tenant: regions are required"),
		},
		{
			name: "empty default assets rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.DefaultAssets = nil
				return p
			}(),
			expectedError: errors.New("tenant: default assets are required"),
		},
		{
			name: "zero max assets rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.MaxAssets = 0
				return p
			}(),
			expectedError: errors.New("tenant: max assets must be positive"),
		},
		{
			name: "negative max assets rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.MaxAssets = -1
				return p
			}(),
			expectedError: errors.New("tenant: max assets must be positive"),
		},
		{
			name: "missing platform tenant id rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.PlatformTenantID = ""
				return p
			}(),
			expectedError: errors.New("tenant: platform tenant id is required"),
		},
		{
			name: "invalid platform tenant id rejected",
			params: func() command.TenantServiceParams {
				p := baseParams
				p.PlatformTenantID = "invalid-uuid"
				return p
			}(),
			expectedError: errors.New("tenant: invalid platform tenant id: ids: invalid tenant id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := command.NewTenantService(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, svc)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}
}
