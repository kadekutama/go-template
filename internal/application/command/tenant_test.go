package command_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

type tenantStore struct {
	mu      sync.Mutex
	tenants map[valueobject.TenantID]entity.TenantData
}

func (s *tenantStore) Create(_ context.Context, tenant entity.TenantData) (entity.TenantData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tenants == nil {
		s.tenants = map[valueobject.TenantID]entity.TenantData{}
	}
	if tenant.ID == "" {
		tenant.ID = valueobject.TenantID(fmt.Sprintf("10000000-0000-4000-8000-%012d", len(s.tenants)+1))
	}
	if _, dup := s.tenants[tenant.ID]; dup {
		return entity.TenantData{}, entity.NewError("TENANT_CONFLICT", "tenant id already exists")
	}
	for _, existing := range s.tenants {
		if existing.Name == tenant.Name {
			return entity.TenantData{}, entity.NewError("TENANT_NAME_TAKEN", "tenant name is taken")
		}
	}
	s.tenants[tenant.ID] = tenant
	return tenant, nil
}

func (s *tenantStore) ListAliases(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	aliases := make([]string, 0, len(s.tenants))
	for _, tenant := range s.tenants {
		if tenant.Alias != "" {
			aliases = append(aliases, tenant.Alias)
		}
	}

	return aliases, nil
}

func (s *tenantStore) FindByID(_ context.Context, id valueobject.TenantID) (entity.TenantData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tenant, ok := s.tenants[id]
	if !ok {
		return entity.TenantData{}, entity.NewError("TENANT_NOT_FOUND", "tenant is unknown")
	}
	return tenant, nil
}

func (s *tenantStore) ListNames(_ context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	names := make([]string, 0, len(s.tenants))
	for _, tenant := range s.tenants {
		names = append(names, tenant.Name)
	}
	return names, nil
}

func (s *tenantStore) ListTenants(_ context.Context) ([]entity.TenantData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []entity.TenantData
	for _, tenant := range s.tenants {
		out = append(out, tenant)
	}
	return out, nil
}

func (s *tenantStore) UpdateSettings(_ context.Context, tenant entity.TenantData, expectedVersion int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, ok := s.tenants[tenant.ID]
	if !ok {
		return entity.NewError("TENANT_NOT_FOUND", "tenant is unknown")
	}
	if stored.Version != expectedVersion {
		return entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry")
	}
	tenant.Version = expectedVersion + 1
	s.tenants[tenant.ID] = tenant
	return nil
}

func tenantSettings() entity.TenantSettings {
	return entity.TenantSettings{
		DefaultCurrency:       "USD",
		Timezone:              "UTC",
		EnabledFeatures:       []string{"transfers"},
		EnabledPaymentMethods: []string{"ACH"},
	}
}

func newTenantService(uow *acctUOW, store *tenantStore, authz *acctAuthz) *command.TenantService {
	return command.NewTenantService(command.TenantServiceParams{
		UoW:           uow,
		Tenants:       store,
		Clock:         acctClock{},
		IDs:           &acctIDs{next: acctTestIDs()},
		Authz:         authz,
		Regions:       []string{"us-east", "eu-west"},
		DefaultAssets: []valueobject.AssetCode{"USD"},
	})
}

func provisionTestTenant() port.ProvisionTenantRequest {
	return port.ProvisionTenantRequest{
		Name: "acme", Region: "us-east", Settings: tenantSettings(),
		IdempotencyKey: "key-1", Actor: "u-1",
	}
}

func TestTenantProvision(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name               string
		req                port.ProvisionTenantRequest
		preload            func(uow *acctUOW, svc *command.TenantService)
		denied             bool
		expectedError      error
		expectedTenants    int
		expectedOutbox     int
		expectedAuthzCalls int
		expectedAlias      string
	}

	testCases := []testCase{
		{
			name:               "provision creates tenant with defaults",
			req:                provisionTestTenant(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      nil,
			expectedTenants:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 1,
			expectedAlias:      "acme",
		},
		{
			name: "duplicate replay returns original single tenant",
			req:  provisionTestTenant(),
			preload: func(_ *acctUOW, svc *command.TenantService) {
				_, err := svc.ProvisionTenant(context.Background(), provisionTestTenant())
				require.NoError(t, err)
			},
			denied:             false,
			expectedError:      nil,
			expectedTenants:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
			expectedAlias:      "acme",
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  provisionTestTenant(),
			preload: func(uow *acctUOW, _ *command.TenantService) {
				req := provisionTestTenant()
				settingsJSON, _ := jsonparser.Marshal(req.Settings)
				fp := command.Fingerprint(req.IdempotencyKey, req.Name, req.Region, string(settingsJSON))
				uow.idem = map[string]acctIdemEntry{
					req.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			denied:             false,
			expectedError:      entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "taken name fails without persisting",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.IdempotencyKey = "key-2"
				return r
			}(),
			preload: func(_ *acctUOW, svc *command.TenantService) {
				_, err := svc.ProvisionTenant(context.Background(), provisionTestTenant())
				require.NoError(t, err)
			},
			denied:             false,
			expectedError:      entity.NewError("TENANT_NAME_DUPLICATE", "tenant name is already taken"),
			expectedTenants:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 2,
		},
		{
			name: "disallowed region fails without persisting",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.Region = "antarctica"
				r.Name = "other"
				r.IdempotencyKey = "key-2"
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      entity.NewError("TENANT_REGION_INVALID", "tenant region is not allowed"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
		},
		{
			name: "missing name fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.Name = ""
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing actor fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.Actor = ""
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      entity.NewError("ACTOR_REQUIRED", "tenant actor is required"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
		{
			name: "missing idempotency key fails envelope validation",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.IdempotencyKey = ""
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
		},
		{
			name:               "denied subject fails before store touch",
			req:                provisionTestTenant(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             true,
			expectedError:      entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 1,
			expectedAlias:      "",
		},
		{
			name: "explicit alias stored",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.Alias = "acme-corp"
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      nil,
			expectedTenants:    1,
			expectedOutbox:     1,
			expectedAuthzCalls: 1,
			expectedAlias:      "acme-corp",
		},
		{
			name: "malformed alias rejected",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.Alias = "-bad!"
				return r
			}(),
			preload:            func(_ *acctUOW, _ *command.TenantService) {},
			denied:             false,
			expectedError:      entity.NewError("TENANT_ALIAS_INVALID", "tenant alias must be lowercase alphanumeric with hyphens"),
			expectedTenants:    0,
			expectedOutbox:     0,
			expectedAuthzCalls: 0,
			expectedAlias:      "",
		},
		{
			name: "taken alias resolves with suffix",
			req: func() port.ProvisionTenantRequest {
				r := provisionTestTenant()
				r.IdempotencyKey = "key-2"
				r.Name = "Acme."
				return r
			}(),
			preload: func(_ *acctUOW, svc *command.TenantService) {
				_, err := svc.ProvisionTenant(context.Background(), provisionTestTenant())
				require.NoError(t, err)
			},
			denied:             false,
			expectedError:      nil,
			expectedTenants:    2,
			expectedOutbox:     2,
			expectedAuthzCalls: 2,
			expectedAlias:      "acme-2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &acctUOW{}
			store := &tenantStore{}
			authz := &acctAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied["u-1|tenant.provision|platform/tenants"] = true
			}
			svc := newTenantService(uow, store, authz)
			tc.preload(uow, svc)
			actualResult, err := svc.ProvisionTenant(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.req.Name, actualResult.Tenant.Name)
				assert.Equal(t, tc.expectedAlias, actualResult.Tenant.Alias)
				assert.Equal(t, "cursor-3", actualResult.Cursor)
			}
			assert.Len(t, store.tenants, tc.expectedTenants)
			assert.Len(t, uow.outbox, tc.expectedOutbox)
			assert.Equal(t, tc.expectedAuthzCalls, authz.calls)
		})
	}
}

func TestTenantSettingsUpdate(t *testing.T) {
	t.Parallel()

	seedTenant := func(t *testing.T, svc *command.TenantService) port.TenantResult {
		t.Helper()
		res, err := svc.ProvisionTenant(context.Background(), provisionTestTenant())
		require.NoError(t, err)
		return res
	}

	type testCase struct {
		name          string
		req           func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest
		preload       func(svc *command.TenantService, tID valueobject.TenantID)
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "current version updates settings",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-1",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "replay returns original settings without re-mutating",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-replay",
					Actor:           "u-1",
				}
			},
			preload: func(svc *command.TenantService, tID valueobject.TenantID) {
				_, err := svc.UpdateTenantSettings(context.Background(), port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-replay",
					Actor:           "u-1",
				})
				require.NoError(t, err)
			},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "stale version conflicts without write",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 99,
					IdempotencyKey:  "key-settings-stale",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("VERSION_CONFLICT", "version mismatch; reload and retry"),
		},
		{
			name: "nonexistent tenant fails",
			req: func(_ valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        valueobject.TenantID("nonexistent-tenant"),
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-notfound",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
		},
		{
			name: "missing tenant ID fails envelope validation",
			req: func(_ valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        "",
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-notenant",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing actor fails envelope validation",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-noactor",
					Actor:           "",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "tenant actor is required"),
		},
		{
			name: "zero expected version fails envelope validation",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 0,
					IdempotencyKey:  "key-settings-zeroversion",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("TENANT_VERSION_INVALID", "expected version must be at least 1"),
		},
		{
			name: "missing idempotency key fails envelope validation",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "tenant command requires an idempotency key"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			req: func(tID valueobject.TenantID) port.UpdateTenantSettingsRequest {
				return port.UpdateTenantSettingsRequest{
					TenantID:        tID,
					Settings:        tenantSettings(),
					ExpectedVersion: 1,
					IdempotencyKey:  "key-settings-denied",
					Actor:           "u-1",
				}
			},
			preload:       func(_ *command.TenantService, _ valueobject.TenantID) {},
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &acctUOW{}
			store := &tenantStore{}
			authz := &acctAuthz{denied: map[string]bool{}}
			svc := newTenantService(uow, store, authz)
			provisioned := seedTenant(t, svc)
			req := tc.req(provisioned.Tenant.ID)
			if tc.denied {
				authz.denied["u-1|tenant.update|tenant/"+provisioned.Tenant.ID.String()] = true
			}
			tc.preload(svc, provisioned.Tenant.ID)
			updated, err := svc.UpdateTenantSettings(context.Background(), req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tenantSettings(), updated.Tenant.Settings)
			}
		})
	}
}
