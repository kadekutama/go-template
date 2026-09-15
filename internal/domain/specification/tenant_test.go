package specification_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestTenantNameValid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		candidate             string
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "valid name passes",
			ctx:                   context.Background(),
			candidate:             "Acme Corp",
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:                  "empty name fails",
			ctx:                   context.Background(),
			candidate:             "",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_NAME_INVALID",
		},
		{
			name:                  "short name fails",
			ctx:                   context.Background(),
			candidate:             "ab",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_NAME_INVALID",
		},
		{
			name:                  "whitespace name fails",
			ctx:                   context.Background(),
			candidate:             "   ",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_NAME_INVALID",
		},
		{
			name:                  "overlong name fails",
			ctx:                   context.Background(),
			candidate:             strings.Repeat("a", 65),
			expectedPassed:        false,
			expectedViolationCode: "TENANT_NAME_INVALID",
		},
		{
			name:                  "control char in name fails",
			ctx:                   context.Background(),
			candidate:             "Acme\x00Corp",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_NAME_INVALID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := specification.TenantNameValid().Evaluate(tc.ctx, tc.candidate)
			assert.Equal(t, tc.expectedPassed, result.Passed())
			if tc.expectedPassed {
				assert.Empty(t, result.Violations)
			} else {
				assert.NotEmpty(t, result.Violations)
				assert.Equal(t, tc.expectedViolationCode, result.Violations[0].Code)
			}
		})
	}
}

func TestTenantRegionAllowed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		allowed               []string
		candidate             string
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "allowed region passes",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1", "eu-west-1"},
			candidate:             "us-east-1",
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:                  "case-insensitive pass",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1"},
			candidate:             "US-EAST-1",
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:                  "padded candidate pass",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1"},
			candidate:             "  us-east-1  ",
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name:                  "disallowed region fails",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1"},
			candidate:             "ap-south-1",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_REGION_INVALID",
		},
		{
			name:                  "empty region fails",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1"},
			candidate:             "",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_REGION_INVALID",
		},
		{
			name:                  "whitespace region fails",
			ctx:                   context.Background(),
			allowed:               []string{"us-east-1"},
			candidate:             "   ",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_REGION_INVALID",
		},
		{
			name:                  "empty allowlist fails",
			ctx:                   context.Background(),
			allowed:               nil,
			candidate:             "us-east-1",
			expectedPassed:        false,
			expectedViolationCode: "TENANT_REGION_INVALID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := specification.TenantRegionAllowed(tc.allowed).Evaluate(tc.ctx, tc.candidate)
			assert.Equal(t, tc.expectedPassed, result.Passed())
			if tc.expectedPassed {
				assert.Empty(t, result.Violations)
			} else {
				assert.NotEmpty(t, result.Violations)
				assert.Equal(t, tc.expectedViolationCode, result.Violations[0].Code)
			}
		})
	}
}

func TestTenantSettingsValid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		candidate             entity.TenantSettings
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "valid settings pass",
			ctx:  context.Background(),
			candidate: entity.TenantSettings{
				DefaultCurrency:       "USD",
				Timezone:              "UTC",
				EnabledFeatures:       []string{"transfers"},
				EnabledPaymentMethods: []string{"card"},
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "missing currency fails",
			ctx:  context.Background(),
			candidate: entity.TenantSettings{
				Timezone: "UTC",
			},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_SETTINGS_INVALID",
		},
		{
			name: "missing timezone fails",
			ctx:  context.Background(),
			candidate: entity.TenantSettings{
				DefaultCurrency: "USD",
			},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_SETTINGS_INVALID",
		},
		{
			name: "whitespace feature fails",
			ctx:  context.Background(),
			candidate: entity.TenantSettings{
				DefaultCurrency: "USD",
				Timezone:        "UTC",
				EnabledFeatures: []string{"  "},
			},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_SETTINGS_INVALID",
		},
		{
			name: "duplicate features fail",
			ctx:  context.Background(),
			candidate: entity.TenantSettings{
				DefaultCurrency: "USD",
				Timezone:        "UTC",
				EnabledFeatures: []string{"transfers", "Transfers"},
			},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_SETTINGS_INVALID",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := specification.TenantSettingsValid().Evaluate(tc.ctx, tc.candidate)
			assert.Equal(t, tc.expectedPassed, result.Passed())
			if tc.expectedPassed {
				assert.Empty(t, result.Violations)
			} else {
				assert.NotEmpty(t, result.Violations)
				assert.Equal(t, tc.expectedViolationCode, result.Violations[0].Code)
			}
		})
	}
}
