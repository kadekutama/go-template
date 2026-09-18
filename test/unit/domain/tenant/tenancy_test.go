// Package tenant_test proves the E05 G2 slice through public domain APIs only:
// onboarding all-or-nothing, hierarchy DAG, FX consolidation, key/subject
// round-trips, and residency/white-label validation.
package tenant_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestTenancySliceOnboarding(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseReq := service.OnboardingRequest{
		TenantID:       "10000000-0000-4000-8000-000000000001",
		LedgerID:       "20000000-0000-4000-8000-000000000001",
		Name:           "Slice Corp",
		Region:         "us-east-1",
		Settings:       entity.TenantSettings{DefaultCurrency: "USD", Timezone: "UTC"},
		Assets:         []valueobject.AssetCode{"USD"},
		ExistingNames:  nil,
		AllowedRegions: []string{"us-east-1"},
		RequestedBy:    "u-1",
		EventID:        "ev-slice-1",
		Now:            at,
	}

	type testCase struct {
		name           string
		req            service.OnboardingRequest
		expectedResult service.OnboardingPlan
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid onboarding request produces complete plan",
			req:  baseReq,
			expectedResult: service.OnboardingPlan{
				Tenant: entity.TenantData{
					ID:        "10000000-0000-4000-8000-000000000001",
					Name:      "Slice Corp",
					Alias:     "slice-corp",
					Region:    "us-east-1",
					Status:    entity.TenantActive,
					Settings:  entity.TenantSettings{DefaultCurrency: "USD", Timezone: "UTC"},
					Version:   1,
					CreatedAt: at,
					UpdatedAt: at,
				},
				LedgerID: "20000000-0000-4000-8000-000000000001",
				Accounts: []service.DefaultAccount{
					{Purpose: service.TenantPurposeOperating, AssetCode: "USD", Number: "1000-USD", Name: "Operating USD", Class: valueobject.ClassLiability},
					{Purpose: service.TenantPurposeFee, AssetCode: "USD", Number: "4000-USD", Name: "Platform Fees USD", Class: valueobject.ClassRevenue},
					{Purpose: service.TenantPurposeSuspense, AssetCode: "USD", Number: "1100-USD", Name: "Suspense USD", Class: valueobject.ClassAsset},
				},
				Key: service.APIKeyDescriptor{
					KeyID:     "key_10000000-0000-4000-8000-000000000001",
					KeyPrefix: "pk_10000000",
					TenantID:  "10000000-0000-4000-8000-000000000001",
					IssuedAt:  at,
				},
				Event: event.TenantCreatedPayload{
					TenantID:      "10000000-0000-4000-8000-000000000001",
					LedgerID:      "20000000-0000-4000-8000-000000000001",
					Name:          "Slice Corp",
					Region:        "us-east-1",
					BaseAssetCode: "USD",
					CreatedAt:     at,
				},
			},
			expectedError: nil,
		},
		{
			name: "disallowed region rejected",
			req: func() service.OnboardingRequest {
				r := baseReq
				r.Region = "eu-central-1"
				return r
			}(),
			expectedResult: service.OnboardingPlan{},
			expectedError:  entity.NewError("TENANT_REGION_INVALID", "tenant region is not allowed"),
		},
		{
			name: "duplicate tenant name rejected",
			req: func() service.OnboardingRequest {
				r := baseReq
				r.ExistingNames = []string{"slice corp"}
				return r
			}(),
			expectedResult: service.OnboardingPlan{},
			expectedError:  entity.NewError("TENANT_NAME_DUPLICATE", "tenant name is already taken"),
		},
		{
			name: "default currency missing from allowed assets rejected",
			req: func() service.OnboardingRequest {
				r := baseReq
				r.Assets = []valueobject.AssetCode{"EUR"}
				return r
			}(),
			expectedResult: service.OnboardingPlan{},
			expectedError:  entity.NewError("ONBOARDING_ASSET_INVALID", "default currency must be included in onboarding assets"),
		},
		{
			name: "whitespace tenant name rejected",
			req: func() service.OnboardingRequest {
				r := baseReq
				r.Name = "   "
				return r
			}(),
			expectedResult: service.OnboardingPlan{},
			expectedError:  entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ValidateOnboarding(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceAggregateLifecycle(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseParams := aggregate.OpenTenantParams{
		ID:         "10000000-0000-4000-8000-000000000001",
		LedgerID:   "20000000-0000-4000-8000-000000000001",
		Name:       "Slice Corp",
		Region:     "us-east-1",
		Settings:   entity.TenantSettings{DefaultCurrency: "USD", Timezone: "UTC"},
		OpenedBy:   "u-1",
		EventID:    "ev-slice-1",
		OccurredAt: at,
	}

	type testCase struct {
		name            string
		p               aggregate.OpenTenantParams
		expectedStatus  entity.TenantStatus
		expectedVersion int64
		expectedEvent   string
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "open active tenant at version 1",
			p:               baseParams,
			expectedStatus:  entity.TenantActive,
			expectedVersion: 1,
			expectedEvent:   "tenant.created.v1",
			expectedError:   nil,
		},
		{
			name: "open tenant with empty id succeeds without events",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.ID = ""
				return p
			}(),
			expectedStatus:  entity.TenantActive,
			expectedVersion: 1,
			expectedEvent:   "",
			expectedError:   nil,
		},
		{
			name: "invalid tenant id rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.ID = "invalid-uuid"
				return p
			}(),
			expectedStatus:  "",
			expectedVersion: 0,
			expectedEvent:   "",
			expectedError:   entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "invalid ledger id rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.LedgerID = "invalid-uuid"
				return p
			}(),
			expectedStatus:  "",
			expectedVersion: 0,
			expectedEvent:   "",
			expectedError:   entity.NewError("LEDGER_ID_INVALID", "ledger id is invalid"),
		},
		{
			name: "missing opened by user id rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.OpenedBy = ""
				return p
			}(),
			expectedStatus:  "",
			expectedVersion: 0,
			expectedEvent:   "",
			expectedError:   entity.NewError("OPENED_BY_REQUIRED", "opened by user id is required"),
		},
		{
			name: "missing event id rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.EventID = ""
				return p
			}(),
			expectedStatus:  "",
			expectedVersion: 0,
			expectedEvent:   "",
			expectedError:   entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name: "zero occurred at time rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.OccurredAt = time.Time{}
				return p
			}(),
			expectedStatus:  "",
			expectedVersion: 0,
			expectedEvent:   "",
			expectedError:   entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := aggregate.OpenTenant(tc.p)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Record().Status)
				assert.Equal(t, tc.expectedVersion, actualResult.Record().Version)
				evts := actualResult.UncommittedEvents()
				if tc.expectedEvent != "" {
					require.Len(t, evts, 1)
					assert.Equal(t, tc.expectedEvent, evts[0].EventType())
				} else {
					require.Empty(t, evts)
				}
			}
		})
	}
}

func TestTenancySliceValidateHierarchy(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		links         []entity.TenantLink
		expectedError error
	}

	testCases := []testCase{
		{
			name: "acyclic multi-level hierarchy passes",
			links: []entity.TenantLink{
				{ParentID: "t-parent", ChildID: "t-sub1"},
				{ParentID: "t-sub1", ChildID: "t-sub2"},
			},
			expectedError: nil,
		},
		{
			name: "self-parent cycle rejected",
			links: []entity.TenantLink{
				{ParentID: "t-self", ChildID: "t-self"},
			},
			expectedError: entity.NewError("HIERARCHY_SELF_PARENT", "tenant cannot parent to itself"),
		},
		{
			name: "two-node cycle rejected",
			links: []entity.TenantLink{
				{ParentID: "t-a", ChildID: "t-b"},
				{ParentID: "t-b", ChildID: "t-a"},
			},
			expectedError: entity.NewError("HIERARCHY_CYCLE", "tenant hierarchy must be acyclic"),
		},
		{
			name: "duplicate links handled safely",
			links: []entity.TenantLink{
				{ParentID: "t-parent", ChildID: "t-sub1"},
				{ParentID: "t-parent", ChildID: "t-sub1"},
			},
			expectedError: nil,
		},
		{
			name: "depth exceeding maximum rejected",
			links: []entity.TenantLink{
				{ParentID: "t-1", ChildID: "t-2"},
				{ParentID: "t-2", ChildID: "t-3"},
				{ParentID: "t-3", ChildID: "t-4"},
				{ParentID: "t-4", ChildID: "t-5"},
				{ParentID: "t-5", ChildID: "t-6"},
			},
			expectedError: entity.NewError("HIERARCHY_DEPTH_EXCEEDED", "tenant hierarchy exceeds depth limit"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := entity.ValidateHierarchy(tc.links)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceValidateTransferGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		grants        map[string]bool
		childTenantID string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "explicit grant present",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "t-child",
			expectedError: nil,
		},
		{
			name:          "missing grant rejected",
			grants:        map[string]bool{"t-other": true},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "grant set to false rejected",
			grants:        map[string]bool{"t-child": false},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "nil grants map rejected",
			grants:        nil,
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant"),
		},
		{
			name:          "whitespace child id rejected",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "   ",
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateTransferGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceValidateReadGrant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		grants        map[string]bool
		childTenantID string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "explicit read grant present",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "t-child",
			expectedError: nil,
		},
		{
			name:          "missing read grant rejected",
			grants:        map[string]bool{"t-other": true},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "read grant set to false rejected",
			grants:        map[string]bool{"t-child": false},
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "nil grants map rejected",
			grants:        nil,
			childTenantID: "t-child",
			expectedError: entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant"),
		},
		{
			name:          "empty child id rejected",
			grants:        map[string]bool{"t-child": true},
			childTenantID: "",
			expectedError: entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateReadGrant(tc.grants, tc.childTenantID)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceHierarchyAndConsolidation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseReq := service.ConsolidationRequest{
		BaseAsset: "USD",
		Children: []service.ChildBalance{
			{TenantID: "t-a", AssetCode: "USD", AmountMinor: 5000},
			{TenantID: "t-b", AssetCode: "EUR", AmountMinor: 10000},
		},
		Rates: map[valueobject.AssetCode]valueobject.FxRate{
			"EUR": {
				ID:          "EURUSD-slice",
				Pair:        valueobject.FxPair{Base: "EUR", Quote: "USD"},
				Numerator:   10850,
				Denominator: 10000,
				Source:      "ecb",
				QuotedAt:    at,
				TTL:         time.Hour,
			},
		},
		At: at,
	}

	type testCase struct {
		name           string
		req            service.ConsolidationRequest
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "multi-currency FX roll-up half-up exactness",
			req:            baseReq,
			expectedResult: int64(15850),
			expectedError:  nil,
		},
		{
			name: "same asset consolidation without conversion",
			req: func() service.ConsolidationRequest {
				r := baseReq
				r.Children = []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: 5000},
					{TenantID: "t-b", AssetCode: "USD", AmountMinor: 10000},
				}
				return r
			}(),
			expectedResult: int64(15000),
			expectedError:  nil,
		},
		{
			name: "missing FX rate rejected",
			req: func() service.ConsolidationRequest {
				r := baseReq
				r.Rates = map[valueobject.AssetCode]valueobject.FxRate{}
				return r
			}(),
			expectedResult: 0,
			expectedError:  entity.NewError("FX_RATE_MISSING", "consolidation requires an FX rate for child asset"),
		},
		{
			name: "stale FX rate rejected",
			req: func() service.ConsolidationRequest {
				r := baseReq
				r.Rates = map[valueobject.AssetCode]valueobject.FxRate{
					"EUR": {
						ID:          "EURUSD-slice",
						Pair:        valueobject.FxPair{Base: "EUR", Quote: "USD"},
						Numerator:   10850,
						Denominator: 10000,
						Source:      "ecb",
						QuotedAt:    at.Add(-2 * time.Hour),
						TTL:         time.Hour,
					},
				}
				return r
			}(),
			expectedResult: 0,
			expectedError:  entity.NewError("FX_RATE_STALE", "fx rate is stale"),
		},
		{
			name: "empty children list rejected",
			req: func() service.ConsolidationRequest {
				r := baseReq
				r.Children = nil
				return r
			}(),
			expectedResult: 0,
			expectedError:  entity.NewError("CONSOLIDATION_EMPTY", "consolidation requires at least one child balance"),
		},
		{
			name: "negative child amount rejected",
			req: func() service.ConsolidationRequest {
				r := baseReq
				r.Children = []service.ChildBalance{
					{TenantID: "t-a", AssetCode: "USD", AmountMinor: -100},
				}
				return r
			}(),
			expectedResult: 0,
			expectedError:  entity.NewError("CONSOLIDATION_AMOUNT_INVALID", "consolidation amount must be non-negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ConsolidateBalances(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationParseBalanceKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		key             string
		expectedTenant  string
		expectedAccount string
		expectedAsset   string
		expectedCursor  string
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "three-segment balance key",
			key:             "balance:t-e05:a-1:USD",
			expectedTenant:  "t-e05",
			expectedAccount: "a-1",
			expectedAsset:   "USD",
			expectedCursor:  "",
			expectedError:   nil,
		},
		{
			name:            "four-segment cursor balance key",
			key:             "balance:t-e05:a-1:USD:cur-99",
			expectedTenant:  "t-e05",
			expectedAccount: "a-1",
			expectedAsset:   "USD",
			expectedCursor:  "cur-99",
			expectedError:   nil,
		},
		{
			name:            "invalid prefix rejected",
			key:             "cache:t-e05:a-1:USD",
			expectedTenant:  "",
			expectedAccount: "",
			expectedAsset:   "",
			expectedCursor:  "",
			expectedError:   entity.NewError("ISOLATION_KEY_INVALID", "balance key must start with 'balance:'"),
		},
		{
			name:            "insufficient segments rejected",
			key:             "balance:t-e05:a-1",
			expectedTenant:  "",
			expectedAccount: "",
			expectedAsset:   "",
			expectedCursor:  "",
			expectedError:   entity.NewError("ISOLATION_KEY_INVALID", "balance key must have 3 or 4 segments"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTenant, actualAccount, actualAsset, actualCursor, err := service.ParseBalanceKey(tc.key)
			assert.Equal(t, tc.expectedTenant, actualTenant)
			assert.Equal(t, tc.expectedAccount, actualAccount)
			assert.Equal(t, tc.expectedAsset, actualAsset)
			assert.Equal(t, tc.expectedCursor, actualCursor)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationParseIdempotencyKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		key               string
		expectedTenant    string
		expectedOperation string
		expectedClientKey string
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "valid idempotency key",
			key:               "idempotency:t-e05:PostLedgerPosting:idem-123",
			expectedTenant:    "t-e05",
			expectedOperation: "PostLedgerPosting",
			expectedClientKey: "idem-123",
			expectedError:     nil,
		},
		{
			name:              "invalid prefix rejected",
			key:               "lock:t-e05:PostLedgerPosting:idem-123",
			expectedTenant:    "",
			expectedOperation: "",
			expectedClientKey: "",
			expectedError:     entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must start with 'idempotency:'"),
		},
		{
			name:              "insufficient segments rejected",
			key:               "idempotency:t-e05:PostLedgerPosting",
			expectedTenant:    "",
			expectedOperation: "",
			expectedClientKey: "",
			expectedError:     entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must have 3 segments"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTenant, actualOp, actualClientKey, err := service.ParseIdempotencyKey(tc.key)
			assert.Equal(t, tc.expectedTenant, actualTenant)
			assert.Equal(t, tc.expectedOperation, actualOp)
			assert.Equal(t, tc.expectedClientKey, actualClientKey)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationParseRateLimitKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		key            string
		expectedTenant string
		expectedUser   string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid rate limit key",
			key:            "ratelimit:t-e05:u-1",
			expectedTenant: "t-e05",
			expectedUser:   "u-1",
			expectedError:  nil,
		},
		{
			name:           "invalid prefix rejected",
			key:            "throttle:t-e05:u-1",
			expectedTenant: "",
			expectedUser:   "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "rate-limit key must start with 'ratelimit:'"),
		},
		{
			name:           "insufficient segments rejected",
			key:            "ratelimit:t-e05",
			expectedTenant: "",
			expectedUser:   "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "rate-limit key must have 2 segments"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTenant, actualUser, err := service.ParseRateLimitKey(tc.key)
			assert.Equal(t, tc.expectedTenant, actualTenant)
			assert.Equal(t, tc.expectedUser, actualUser)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationParseSubject(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		subject        string
		expectedTenant string
		expectedEvent  string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid event subject",
			subject:        "ledger.t-e05.tenant.created.v1",
			expectedTenant: "t-e05",
			expectedEvent:  "tenant.created.v1",
			expectedError:  nil,
		},
		{
			name:           "invalid prefix rejected",
			subject:        "events.t-e05.tenant.created.v1",
			expectedTenant: "",
			expectedEvent:  "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject must start with 'ledger.'"),
		},
		{
			name:           "insufficient segments rejected",
			subject:        "ledger.t-e05",
			expectedTenant: "",
			expectedEvent:  "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject must carry tenant and event type"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTenant, actualEvent, err := service.ParseSubject(tc.subject)
			assert.Equal(t, tc.expectedTenant, actualTenant)
			assert.Equal(t, tc.expectedEvent, actualEvent)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationRequireTenant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenantID      string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid tenant id",
			tenantID:      "t-e05",
			expectedError: nil,
		},
		{
			name:          "empty tenant id rejected",
			tenantID:      "",
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name:          "whitespace tenant id rejected",
			tenantID:      "   ",
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.RequireTenant(tc.tenantID)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationResidencySettings(t *testing.T) {
	t.Parallel()

	baseSettings := service.ResidencySettings{
		Region:    "us-east-1",
		DBPointer: "db-1",
	}

	type testCase struct {
		name          string
		r             service.ResidencySettings
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid residency settings",
			r:             baseSettings,
			expectedError: nil,
		},
		{
			name: "empty region rejected",
			r: func() service.ResidencySettings {
				s := baseSettings
				s.Region = ""
				return s
			}(),
			expectedError: entity.NewError("RESIDENCY_REGION_REQUIRED", "residency region is required"),
		},
		{
			name: "whitespace region rejected",
			r: func() service.ResidencySettings {
				s := baseSettings
				s.Region = "   "
				return s
			}(),
			expectedError: entity.NewError("RESIDENCY_REGION_REQUIRED", "residency region is required"),
		},
		{
			name: "empty db pointer rejected",
			r: func() service.ResidencySettings {
				s := baseSettings
				s.DBPointer = ""
				return s
			}(),
			expectedError: entity.NewError("RESIDENCY_DB_REQUIRED", "residency db pointer is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.r.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationWhiteLabelSettings(t *testing.T) {
	t.Parallel()

	baseSettings := service.WhiteLabelSettings{
		BrandName: "Slice",
		Domains:   []string{"pay.slice.example.com"},
	}

	type testCase struct {
		name          string
		w             service.WhiteLabelSettings
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid white label settings",
			w:             baseSettings,
			expectedError: nil,
		},
		{
			name: "missing brand name rejected",
			w: func() service.WhiteLabelSettings {
				s := baseSettings
				s.BrandName = ""
				return s
			}(),
			expectedError: entity.NewError("WHITELABEL_BRAND_REQUIRED", "brand name is required"),
		},
		{
			name: "bare hostname required rejected",
			w: func() service.WhiteLabelSettings {
				s := baseSettings
				s.Domains = []string{"https://pay.slice.example.com/path"}
				return s
			}(),
			expectedError: entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain must be a bare hostname"),
		},
		{
			name: "whitespace domain entry rejected",
			w: func() service.WhiteLabelSettings {
				s := baseSettings
				s.Domains = []string{"pay slice com"}
				return s
			}(),
			expectedError: entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain must be a bare hostname"),
		},
		{
			name: "invalid color rejected",
			w: func() service.WhiteLabelSettings {
				s := baseSettings
				s.PrimaryColor = "blue"
				return s
			}(),
			expectedError: entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be hex"),
		},
		{
			name: "non-https logo rejected",
			w: func() service.WhiteLabelSettings {
				s := baseSettings
				s.LogoURL = "http://cdn.slice.example.com/logo.png"
				return s
			}(),
			expectedError: entity.NewError("WHITELABEL_LOGO_INVALID", "logo url must be an https url without spaces"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.w.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenancySliceIsolationRLSPolicyMatrix(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		expectedLength int
		expectedPolicy service.RLSPolicy
	}

	testCases := []testCase{
		{
			name:           "tenants select is self scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "tenants",
				Operation: "SELECT",
				Scope:     service.RLSScopeSelf,
			},
		},
		{
			name:           "ledgers select is tenant scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "ledgers",
				Operation: "SELECT",
				Scope:     service.RLSScopeTenant,
			},
		},
		{
			name:           "accounts insert is tenant scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "accounts",
				Operation: "INSERT",
				Scope:     service.RLSScopeTenant,
			},
		},
		{
			name:           "postings insert is tenant scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "postings",
				Operation: "INSERT",
				Scope:     service.RLSScopeTenant,
			},
		},
		{
			name:           "asset_registry select is shared-read",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "asset_registry",
				Operation: "SELECT",
				Scope:     service.RLSScopeSharedRead,
			},
		},
		{
			name:           "asset_registry insert is service scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "asset_registry",
				Operation: "INSERT",
				Scope:     service.RLSScopeService,
			},
		},
		{
			name:           "inbox_receipts select is service scoped",
			expectedLength: 44,
			expectedPolicy: service.RLSPolicy{
				Table:     "inbox_receipts",
				Operation: "SELECT",
				Scope:     service.RLSScopeService,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.RLSPolicyMatrix()
			assert.Len(t, actualResult, tc.expectedLength)
			assert.Contains(t, actualResult, tc.expectedPolicy)
		})
	}
}
