package service_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func baseOnboardingRequest() service.OnboardingRequest {
	return service.OnboardingRequest{
		TenantID: "10000000-0000-4000-8000-000000000001",
		LedgerID: "20000000-0000-4000-8000-000000000001",
		Name:     "Acme Corp",
		Region:   "us-east-1",
		Settings: entity.TenantSettings{
			DefaultCurrency: "USD",
			Timezone:        "UTC",
		},
		Assets:         []valueobject.AssetCode{"USD", "EUR"},
		ExistingNames:  []string{"Other Corp"},
		AllowedRegions: []string{"us-east-1", "eu-west-1"},
		RequestedBy:    "30000000-0000-4000-8000-000000000001",
		EventID:        "ev-1",
		Now:            time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
	}
}

func TestValidateOnboarding(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		req           service.OnboardingRequest
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid onboarding with two assets",
			req:           baseOnboardingRequest(),
			expectedCount: 6,
			expectedError: nil,
		},
		{
			name: "duplicate name rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.Name = "other corp"
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("TENANT_NAME_DUPLICATE", "tenant name is already taken"),
		},
		{
			name: "disallowed region rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.Region = "ap-south-1"
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("TENANT_REGION_INVALID", "tenant region is not allowed"),
		},
		{
			name: "missing tenant id permitted before persistence",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.TenantID = ""
				return r
			}(),
			expectedCount: 6,
			expectedError: nil,
		},
		{
			name: "whitespace tenant id rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.TenantID = "   "
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "invalid tenant id format",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.TenantID = "not-a-uuid"
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "missing ledger id permitted before persistence",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.LedgerID = ""
				return r
			}(),
			expectedCount: 6,
			expectedError: nil,
		},
		{
			name: "whitespace ledger id rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.LedgerID = "   "
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("LEDGER_ID_INVALID", "ledger id is invalid"),
		},
		{
			name: "invalid ledger id format",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.LedgerID = "not-a-uuid"
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("LEDGER_ID_INVALID", "ledger id is invalid"),
		},
		{
			name: "invalid settings rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.Settings.Timezone = ""
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is required"),
		},
		{
			name: "default currency not in assets rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.Settings.DefaultCurrency = "GBP"
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSET_INVALID", "default currency must be included in onboarding assets"),
		},
		{
			name: "missing actor",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.RequestedBy = ""
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ACTOR_REQUIRED", "onboarding actor is required"),
		},
		{
			name: "whitespace actor",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.RequestedBy = "   "
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ACTOR_REQUIRED", "onboarding actor is required"),
		},
		{
			name: "missing event id",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.EventID = ""
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_EVENT_REQUIRED", "onboarding event id is required"),
		},
		{
			name: "whitespace event id",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.EventID = "   "
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_EVENT_REQUIRED", "onboarding event id is required"),
		},
		{
			name: "zero time rejected",
			req: func() service.OnboardingRequest {
				r := baseOnboardingRequest()
				r.Now = time.Time{}
				return r
			}(),
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_TIME_REQUIRED", "onboarding time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			plan, err := service.ValidateOnboarding(tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Len(t, plan.Accounts, tc.expectedCount)
				assert.Equal(t, tc.req.TenantID.String(), plan.Tenant.ID.String())
				assert.Equal(t, tc.req.LedgerID.String(), plan.LedgerID)
				assert.Equal(t, "Acme Corp", plan.Event.Name)
				if tc.req.TenantID.String() != "" {
					assert.NotEmpty(t, plan.Key.KeyID)
				}
			} else {
				assert.Empty(t, plan.Accounts)
				assert.Empty(t, plan.Tenant.ID)
			}
		})
	}
}

func TestValidateOnboardingPlanIsolatedFromRequest(t *testing.T) {
	t.Parallel()

	baseReq := baseOnboardingRequest()
	baseReq.Settings.EnabledFeatures = []string{"transfers"}
	baseReq.Settings.EnabledPaymentMethods = []string{"ach"}

	plan, err := service.ValidateOnboarding(baseReq)
	require.NoError(t, err)
	baseReq.Settings.EnabledFeatures[0] = "MUTATED"
	baseReq.Settings.EnabledPaymentMethods[0] = "MUTATED"
	assert.Equal(t, []string{"transfers"}, plan.Tenant.Settings.EnabledFeatures)
	assert.Equal(t, []string{"ach"}, plan.Tenant.Settings.EnabledPaymentMethods)
}

func TestDefaultAccountsForAssets(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		assets        []valueobject.AssetCode
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "single asset yields three accounts",
			assets:        []valueobject.AssetCode{"USD"},
			expectedCount: 3,
			expectedError: nil,
		},
		{
			name:          "two assets yield six accounts",
			assets:        []valueobject.AssetCode{"USD", "EUR"},
			expectedCount: 6,
			expectedError: nil,
		},
		{
			name:          "maximum eight assets passes",
			assets:        []valueobject.AssetCode{"A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8"},
			expectedCount: 24,
			expectedError: nil,
		},
		{
			name:          "empty assets rejected",
			assets:        nil,
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSETS_REQUIRED", "onboarding requires at least one asset"),
		},
		{
			name:          "empty asset code rejected",
			assets:        []valueobject.AssetCode{""},
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding asset code is required"),
		},
		{
			name:          "whitespace asset code rejected",
			assets:        []valueobject.AssetCode{"   "},
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding asset code is required"),
		},
		{
			name:          "duplicate assets rejected",
			assets:        []valueobject.AssetCode{"USD", "USD"},
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding assets must be unique"),
		},
		{
			name:          "duplicate assets with whitespace rejected",
			assets:        []valueobject.AssetCode{"USD", " USD "},
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding assets must be unique"),
		},
		{
			name:          "too many assets rejected",
			assets:        []valueobject.AssetCode{"A1", "A2", "A3", "A4", "A5", "A6", "A7", "A8", "A9"},
			expectedCount: 0,
			expectedError: entity.NewError("ONBOARDING_ASSETS_INVALID", "onboarding requests too many assets"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accounts, err := service.DefaultAccountsForAssets(tc.assets)
			assert.Equal(t, tc.expectedError, err)
			assert.Len(t, accounts, tc.expectedCount)
			if tc.expectedError == nil {
				purposes := map[string]bool{}
				for _, a := range accounts {
					purposes[a.Purpose] = true
					require.NotEmpty(t, a.Number)
					require.NotEmpty(t, a.Name)
				}
				assert.True(t, purposes[service.TenantPurposeOperating])
				assert.True(t, purposes[service.TenantPurposeFee])
				assert.True(t, purposes[service.TenantPurposeSuspense])
			}
		})
	}
}

func TestIsRegionAllowed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		region         string
		allowed        []string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "allowed region",
			region:         "us-east-1",
			allowed:        []string{"us-east-1", "eu-west-1"},
			expectedResult: true,
		},
		{
			name:           "case-insensitive match",
			region:         "US-EAST-1",
			allowed:        []string{"us-east-1"},
			expectedResult: true,
		},
		{
			name:           "padded region match",
			region:         "  us-east-1  ",
			allowed:        []string{"us-east-1"},
			expectedResult: true,
		},
		{
			name:           "padded allowlist entry match",
			region:         "us-east-1",
			allowed:        []string{"  us-east-1  "},
			expectedResult: true,
		},
		{
			name:           "disallowed region",
			region:         "ap-south-1",
			allowed:        []string{"us-east-1"},
			expectedResult: false,
		},
		{
			name:           "empty region",
			region:         "",
			allowed:        []string{"us-east-1"},
			expectedResult: false,
		},
		{
			name:           "whitespace region",
			region:         "   ",
			allowed:        []string{"us-east-1"},
			expectedResult: false,
		},
		{
			name:           "empty allowlist",
			region:         "us-east-1",
			allowed:        nil,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.IsRegionAllowed(tc.region, tc.allowed)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestIsNameTaken(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenantName     string
		existing       []string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "exact duplicate",
			tenantName:     "Acme",
			existing:       []string{"Acme"},
			expectedResult: true,
		},
		{
			name:           "case-insensitive duplicate",
			tenantName:     "acme",
			existing:       []string{"ACME"},
			expectedResult: true,
		},
		{
			name:           "padded name duplicate",
			tenantName:     "  acme  ",
			existing:       []string{"ACME"},
			expectedResult: true,
		},
		{
			name:           "padded existing entry duplicate",
			tenantName:     "acme",
			existing:       []string{"  ACME  "},
			expectedResult: true,
		},
		{
			name:           "unique name",
			tenantName:     "Beta",
			existing:       []string{"Acme"},
			expectedResult: false,
		},
		{
			name:           "empty name never taken",
			tenantName:     "",
			existing:       []string{"Acme"},
			expectedResult: false,
		},
		{
			name:           "whitespace name never taken",
			tenantName:     "   ",
			existing:       []string{"Acme"},
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.IsNameTaken(tc.tenantName, tc.existing)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestDeriveTenantAlias(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		requested     string
		tenantName    string
		taken         []string
		expectedAlias string
		expectedError error
	}

	exhausted := func() []string {
		taken := []string{"acme"}
		for attempt := 2; attempt <= 100; attempt++ {
			taken = append(taken, "acme-"+strconv.Itoa(attempt))
		}
		return taken
	}()

	testCases := []testCase{
		{
			name:          "explicit alias wins",
			requested:     "acme-corp",
			tenantName:    "Acme Corporation",
			taken:         nil,
			expectedAlias: "acme-corp",
			expectedError: nil,
		},
		{
			name:          "malformed explicit alias rejected",
			requested:     "-Bad_Alias!",
			tenantName:    "Acme Corporation",
			taken:         nil,
			expectedAlias: "",
			expectedError: entity.NewError("TENANT_ALIAS_INVALID", "tenant alias must be lowercase alphanumeric with hyphens"),
		},
		{
			name:          "derived from display name",
			requested:     "",
			tenantName:    "Acme Corporation",
			taken:         nil,
			expectedAlias: "acme-corporation",
			expectedError: nil,
		},
		{
			name:          "collision resolves with suffix",
			requested:     "",
			tenantName:    "Acme Corporation",
			taken:         []string{"acme-corporation"},
			expectedAlias: "acme-corporation-2",
			expectedError: nil,
		},
		{
			name:          "exhausted namespace fails closed",
			requested:     "",
			tenantName:    "Acme",
			taken:         exhausted,
			expectedAlias: "",
			expectedError: entity.NewError("TENANT_ALIAS_TAKEN", "tenant alias namespace exhausted"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			alias, err := service.DeriveTenantAlias(tc.requested, tc.tenantName, tc.taken)
			assert.Equal(t, tc.expectedError, err)
			assert.Equal(t, tc.expectedAlias, alias)
		})
	}
}
