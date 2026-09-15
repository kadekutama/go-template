package service_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
)

func TestBuildBalanceKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		account        string
		asset          string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid balance key",
			tenant:         "t-1",
			account:        "a-1",
			asset:          "USD",
			expectedResult: "balance:t-1:a-1:USD",
			expectedError:  nil,
		},
		{
			name:           "empty tenant rejected",
			tenant:         "",
			account:        "a-1",
			asset:          "USD",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required"),
		},
		{
			name:           "colon in segment rejected",
			tenant:         "t:1",
			account:        "a-1",
			asset:          "USD",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain ':'"),
		},
		{
			name:           "overlong segment rejected",
			tenant:         strings.Repeat("a", 129),
			account:        "a-1",
			asset:          "USD",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is too long"),
		},
		{
			name:           "control character in segment rejected",
			tenant:         "t\x011",
			account:        "a-1",
			asset:          "USD",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain control characters"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildBalanceKey(tc.tenant, tc.account, tc.asset)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBuildBalanceCursorKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		account        string
		asset          string
		cursor         string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid cursor key",
			tenant:         "t-1",
			account:        "a-1",
			asset:          "USD",
			cursor:         "42",
			expectedResult: "balance:t-1:a-1:USD:42",
			expectedError:  nil,
		},
		{
			name:           "empty cursor rejected",
			tenant:         "t-1",
			account:        "a-1",
			asset:          "USD",
			cursor:         "",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildBalanceCursorKey(tc.tenant, tc.account, tc.asset, tc.cursor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseBalanceKey(t *testing.T) {
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
			name:            "parse three-segment key",
			key:             "balance:t-1:a-1:USD",
			expectedTenant:  "t-1",
			expectedAccount: "a-1",
			expectedAsset:   "USD",
			expectedCursor:  "",
			expectedError:   nil,
		},
		{
			name:            "parse four-segment key",
			key:             "balance:t-1:a-1:USD:42",
			expectedTenant:  "t-1",
			expectedAccount: "a-1",
			expectedAsset:   "USD",
			expectedCursor:  "42",
			expectedError:   nil,
		},
		{
			name:            "wrong prefix rejected",
			key:             "cache:t-1:a-1:USD",
			expectedTenant:  "",
			expectedAccount: "",
			expectedAsset:   "",
			expectedCursor:  "",
			expectedError:   entity.NewError("ISOLATION_KEY_INVALID", "balance key must start with 'balance:'"),
		},
		{
			name:            "bad segment count rejected",
			key:             "balance:t-1:a-1",
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

func TestBuildIdempotencyKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		operation      string
		key            string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid idempotency key",
			tenant:         "t-1",
			operation:      "PostLedgerPosting",
			key:            "abc-123",
			expectedResult: "idempotency:t-1:PostLedgerPosting:abc-123",
			expectedError:  nil,
		},
		{
			name:           "empty operation rejected",
			tenant:         "t-1",
			operation:      "",
			key:            "abc-123",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildIdempotencyKey(tc.tenant, tc.operation, tc.key)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseIdempotencyKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		key               string
		expectedTenant    string
		expectedOperation string
		expectedKey       string
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "round-trip parse",
			key:               "idempotency:t-1:PostLedgerPosting:abc-123",
			expectedTenant:    "t-1",
			expectedOperation: "PostLedgerPosting",
			expectedKey:       "abc-123",
			expectedError:     nil,
		},
		{
			name:              "wrong prefix rejected",
			key:               "idempotent:t-1:op:key",
			expectedTenant:    "",
			expectedOperation: "",
			expectedKey:       "",
			expectedError:     entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must start with 'idempotency:'"),
		},
		{
			name:              "bad segment count rejected",
			key:               "idempotency:t-1:op",
			expectedTenant:    "",
			expectedOperation: "",
			expectedKey:       "",
			expectedError:     entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must have 3 segments"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualTenant, actualOperation, actualKey, err := service.ParseIdempotencyKey(tc.key)
			assert.Equal(t, tc.expectedTenant, actualTenant)
			assert.Equal(t, tc.expectedOperation, actualOperation)
			assert.Equal(t, tc.expectedKey, actualKey)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBuildRateLimitKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		user           string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid rate-limit key",
			tenant:         "t-1",
			user:           "u-1",
			expectedResult: "ratelimit:t-1:u-1",
			expectedError:  nil,
		},
		{
			name:           "empty user rejected",
			tenant:         "t-1",
			user:           "",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildRateLimitKey(tc.tenant, tc.user)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseRateLimitKey(t *testing.T) {
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
			name:           "round-trip parse",
			key:            "ratelimit:t-1:u-1",
			expectedTenant: "t-1",
			expectedUser:   "u-1",
			expectedError:  nil,
		},
		{
			name:           "wrong prefix rejected",
			key:            "ratelimited:t-1:u-1",
			expectedTenant: "",
			expectedUser:   "",
			expectedError:  entity.NewError("ISOLATION_KEY_INVALID", "rate-limit key must start with 'ratelimit:'"),
		},
		{
			name:           "bad segment count rejected",
			key:            "ratelimit:t-1",
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

func TestBuildSubject(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		eventType      string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid subject",
			tenant:         "t-1",
			eventType:      "account.created.v1",
			expectedResult: "ledger.t-1.account.created.v1",
			expectedError:  nil,
		},
		{
			name:           "empty tenant rejected",
			tenant:         "",
			eventType:      "account.created.v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is required"),
		},
		{
			name:           "unversioned event rejected",
			tenant:         "t-1",
			eventType:      "created",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type must be versioned"),
		},
		{
			name:           "tenant with dot rejected",
			tenant:         "t.1",
			eventType:      "account.created.v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant must not contain '.' or ':'"),
		},
		{
			name:           "tenant with colon rejected",
			tenant:         "t:1",
			eventType:      "account.created.v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant must not contain '.' or ':'"),
		},
		{
			name:           "tenant with whitespace rejected",
			tenant:         "t 1",
			eventType:      "account.created.v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant must not contain whitespace"),
		},
		{
			name:           "overlong tenant rejected",
			tenant:         strings.Repeat("t", 65),
			eventType:      "account.created.v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is too long"),
		},
		{
			name:           "event type with whitespace rejected",
			tenant:         "t-1",
			eventType:      "account.created .v1",
			expectedResult: "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type must not contain whitespace"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildSubject(tc.tenant, tc.eventType)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseSubject(t *testing.T) {
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
			name:           "valid subject",
			subject:        "ledger.t-1.account.created.v1",
			expectedTenant: "t-1",
			expectedEvent:  "account.created.v1",
			expectedError:  nil,
		},
		{
			name:           "wrong prefix rejected",
			subject:        "stream.t-1.account.created.v1",
			expectedTenant: "",
			expectedEvent:  "",
			expectedError:  entity.NewError("ISOLATION_SUBJECT_INVALID", "subject must start with 'ledger.'"),
		},
		{
			name:           "missing event rejected",
			subject:        "ledger.t-1",
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

func TestIsolationRoundTrip(t *testing.T) {
	t.Parallel()

	balanceKey, err := service.BuildBalanceKey("t-1", "a-1", "USD")
	require.NoError(t, err)
	tenant, account, asset, cursor, err := service.ParseBalanceKey(balanceKey)
	require.NoError(t, err)
	assert.Equal(t, "t-1", tenant)
	assert.Equal(t, "a-1", account)
	assert.Equal(t, "USD", asset)
	assert.Empty(t, cursor)

	subject, err := service.BuildSubject("t-1", "tenant.created.v1")
	require.NoError(t, err)
	parsedTenant, parsedEvent, err := service.ParseSubject(subject)
	require.NoError(t, err)
	assert.Equal(t, "t-1", parsedTenant)
	assert.Equal(t, "tenant.created.v1", parsedEvent)
}

func TestRLSPolicyMatrix(t *testing.T) {
	t.Parallel()

	matrix := service.RLSPolicyMatrix()
	scopes := map[string]map[string]string{}
	for _, row := range matrix {
		if scopes[row.Table] == nil {
			scopes[row.Table] = map[string]string{}
		}
		scopes[row.Table][row.Operation] = row.Scope
	}
	for _, table := range []string{"tenants", "asset_registry", "ledgers", "accounts", "postings", "entries", "balance_checkpoints", "holds", "idempotency_records", "outbox_events", "inbox_receipts"} {
		assert.Contains(t, scopes, table, "matrix must cover %s", table)
	}
	for _, operation := range []string{"SELECT", "INSERT", "UPDATE", "DELETE"} {
		assert.Equal(t, service.RLSScopeSelf, scopes["tenants"][operation], "tenants rows are self-scoped")
		assert.Equal(t, service.RLSScopeTenant, scopes["ledgers"][operation], "ledgers rows are tenant-scoped")
		assert.Equal(t, service.RLSScopeService, scopes["inbox_receipts"][operation], "inbox has no tenant column and stays service-scoped")
	}
	assert.Equal(t, service.RLSScopeSharedRead, scopes["asset_registry"]["SELECT"], "asset registry is globally readable")
	assert.Equal(t, service.RLSScopeService, scopes["asset_registry"]["INSERT"], "asset registry writes stay service-gated")
}

func TestRequireTenant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenant        string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid tenant",
			tenant:        "t-1",
			expectedError: nil,
		},
		{
			name:          "empty tenant fails closed",
			tenant:        "",
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name:          "whitespace tenant fails closed",
			tenant:        "   ",
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.RequireTenant(tc.tenant)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestResidencySettingsValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		settings      service.ResidencySettings
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid residency",
			settings: service.ResidencySettings{
				Region:    "us-east-1",
				DBPointer: "postgres-primary-1",
			},
			expectedError: nil,
		},
		{
			name: "missing region",
			settings: service.ResidencySettings{
				DBPointer: "db-1",
			},
			expectedError: entity.NewError("RESIDENCY_REGION_REQUIRED", "residency region is required"),
		},
		{
			name: "missing db pointer",
			settings: service.ResidencySettings{
				Region: "us-east-1",
			},
			expectedError: entity.NewError("RESIDENCY_DB_REQUIRED", "residency db pointer is required"),
		},
		{
			name: "db pointer with space rejected",
			settings: service.ResidencySettings{
				Region:    "us-east-1",
				DBPointer: "db primary",
			},
			expectedError: entity.NewError("RESIDENCY_DB_INVALID", "residency db pointer must not contain spaces"),
		},
		{
			name: "db pointer with tab rejected",
			settings: service.ResidencySettings{
				Region:    "us-east-1",
				DBPointer: "db\tprimary",
			},
			expectedError: entity.NewError("RESIDENCY_INVALID", "residency settings must not contain control characters"),
		},
		{
			name: "overlong region rejected",
			settings: service.ResidencySettings{
				Region:    strings.Repeat("r", 33),
				DBPointer: "db-1",
			},
			expectedError: entity.NewError("RESIDENCY_REGION_INVALID", "residency region is too long"),
		},
		{
			name: "overlong db pointer rejected",
			settings: service.ResidencySettings{
				Region:    "us-east-1",
				DBPointer: strings.Repeat("p", 257),
			},
			expectedError: entity.NewError("RESIDENCY_DB_INVALID", "residency db pointer is too long"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestWhiteLabelSettingsValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		settings      service.WhiteLabelSettings
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid white-label",
			settings: service.WhiteLabelSettings{
				BrandName: "Acme",
				Domains:   []string{"pay.acme.com"},
			},
			expectedError: nil,
		},
		{
			name: "missing brand",
			settings: service.WhiteLabelSettings{
				Domains: []string{"pay.acme.com"},
			},
			expectedError: entity.NewError("WHITELABEL_BRAND_REQUIRED", "brand name is required"),
		},
		{
			name: "bare hostname required",
			settings: service.WhiteLabelSettings{
				BrandName: "Acme",
				Domains:   []string{"https://pay.acme.com/path"},
			},
			expectedError: entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain must be a bare hostname"),
		},
		{
			name: "valid color and logo accepted",
			settings: service.WhiteLabelSettings{
				BrandName:    "Acme",
				Domains:      []string{"pay.acme.com"},
				LogoURL:      "https://cdn.acme.com/logo.png",
				PrimaryColor: "#0A1B2C",
			},
			expectedError: nil,
		},
		{
			name: "bad color rejected",
			settings: service.WhiteLabelSettings{
				BrandName:    "Acme",
				Domains:      []string{"pay.acme.com"},
				PrimaryColor: "red",
			},
			expectedError: entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be hex"),
		},
		{
			name: "short hex rejected",
			settings: service.WhiteLabelSettings{
				BrandName:    "Acme",
				Domains:      []string{"pay.acme.com"},
				PrimaryColor: "#12345",
			},
			expectedError: entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be #RGB or #RRGGBB"),
		},
		{
			name: "non-https logo rejected",
			settings: service.WhiteLabelSettings{
				BrandName: "Acme",
				Domains:   []string{"pay.acme.com"},
				LogoURL:   "http://cdn.acme.com/logo.png",
			},
			expectedError: entity.NewError("WHITELABEL_LOGO_INVALID", "logo url must be an https url without spaces"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
