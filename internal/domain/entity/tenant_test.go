package entity_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestTenantDataValidate(t *testing.T) {
	t.Parallel()

	baseTenant := entity.TenantData{
		ID:     "t-1",
		Name:   "Acme Corp",
		Region: "us-east-1",
		Status: entity.TenantActive,
		Settings: entity.TenantSettings{
			DefaultCurrency: "USD",
			Timezone:        "UTC",
		},
		Version:   1,
		CreatedAt: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC),
	}

	type testCase struct {
		name          string
		tenant        entity.TenantData
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid tenant",
			tenant:        baseTenant,
			expectedError: nil,
		},
		{
			name: "missing id",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.ID = ""
				return d
			}(),
			expectedError: entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"),
		},
		{
			name: "whitespace id",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.ID = "   "
				return d
			}(),
			expectedError: entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing name",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Name = "   "
				return d
			}(),
			expectedError: entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
		{
			name: "short name",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Name = "ab"
				return d
			}(),
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must be at least 3 characters"),
		},
		{
			name: "overlong name",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Name = strings.Repeat("a", 65)
				return d
			}(),
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must be at most 64 characters"),
		},
		{
			name: "name with control chars",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Name = "Acme\x00Corp"
				return d
			}(),
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must not contain control characters"),
		},
		{
			name: "missing region",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Region = "  "
				return d
			}(),
			expectedError: entity.NewError("TENANT_REGION_REQUIRED", "tenant region is required"),
		},
		{
			name: "overlong region",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Region = strings.Repeat("r", 33)
				return d
			}(),
			expectedError: entity.NewError("TENANT_REGION_INVALID", "tenant region is too long"),
		},
		{
			name: "region with internal spaces",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Region = "us east 1"
				return d
			}(),
			expectedError: entity.NewError("TENANT_REGION_INVALID", "tenant region must not contain whitespace"),
		},
		{
			name: "region with control characters",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Region = "us-east\x001"
				return d
			}(),
			expectedError: entity.NewError("TENANT_REGION_INVALID", "tenant region must not contain control characters"),
		},
		{
			name: "invalid status",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Status = "WEIRD"
				return d
			}(),
			expectedError: entity.NewError("TENANT_STATUS_INVALID", "tenant status is invalid"),
		},
		{
			name: "empty status",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Status = ""
				return d
			}(),
			expectedError: entity.NewError("TENANT_STATUS_INVALID", "tenant status is invalid"),
		},
		{
			name: "missing currency",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Settings.DefaultCurrency = ""
				return d
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "default currency is required"),
		},
		{
			name: "zero version",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Version = 0
				return d
			}(),
			expectedError: entity.NewError("TENANT_VERSION_INVALID", "version starts at 1"),
		},
		{
			name: "negative version",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.Version = -1
				return d
			}(),
			expectedError: entity.NewError("TENANT_VERSION_INVALID", "version starts at 1"),
		},
		{
			name: "zero created at",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.CreatedAt = time.Time{}
				return d
			}(),
			expectedError: entity.NewError("TENANT_TIME_REQUIRED", "tenant timestamps are required"),
		},
		{
			name: "zero updated at",
			tenant: func() entity.TenantData {
				d := baseTenant
				d.UpdatedAt = time.Time{}
				return d
			}(),
			expectedError: entity.NewError("TENANT_TIME_REQUIRED", "tenant timestamps are required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tenant.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateTenantName(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenantName    string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid name",
			tenantName:    "Acme Corp",
			expectedError: nil,
		},
		{
			name:          "exact 3 chars boundary",
			tenantName:    "ABC",
			expectedError: nil,
		},
		{
			name:          "exact 64 chars boundary",
			tenantName:    strings.Repeat("a", 64),
			expectedError: nil,
		},
		{
			name:          "empty name",
			tenantName:    "",
			expectedError: entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
		{
			name:          "whitespace name",
			tenantName:    "   ",
			expectedError: entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
		{
			name:          "short name",
			tenantName:    "ab",
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must be at least 3 characters"),
		},
		{
			name:          "overlong name",
			tenantName:    strings.Repeat("a", 65),
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must be at most 64 characters"),
		},
		{
			name:          "control characters rejected",
			tenantName:    "Acme\x00Corp",
			expectedError: entity.NewError("TENANT_NAME_INVALID", "tenant name must not contain control characters"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := entity.ValidateTenantName(tc.tenantName)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenantSettingsValidate(t *testing.T) {
	t.Parallel()

	baseSettings := entity.TenantSettings{
		DefaultCurrency: valueobject.AssetCode("USD"),
		Timezone:        "UTC",
	}

	type testCase struct {
		name          string
		settings      entity.TenantSettings
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid settings",
			settings:      baseSettings,
			expectedError: nil,
		},
		{
			name: "missing currency",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.DefaultCurrency = ""
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "default currency is required"),
		},
		{
			name: "missing timezone",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = ""
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is required"),
		},
		{
			name: "whitespace timezone",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = "   "
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is required"),
		},
		{
			name: "timezone with leading space",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = " UTC"
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone must not contain whitespace"),
		},
		{
			name: "timezone with internal space",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = "US Eastern"
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone must not contain whitespace"),
		},
		{
			name: "timezone overlong",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = strings.Repeat("z", 65)
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is too long"),
		},
		{
			name: "timezone with control chars",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.Timezone = "UTC\x00"
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone must not contain control characters"),
		},
		{
			name: "duplicate features rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{"transfers", "transfers"}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entries must be unique"),
		},
		{
			name: "duplicate features case-insensitive rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{"transfers", "TRANSFERS"}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entries must be unique"),
		},
		{
			name: "feature with whitespace padding rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{" transfers "}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entry must not contain leading or trailing whitespace"),
		},
		{
			name: "empty feature entry rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{""}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entries must be non-empty"),
		},
		{
			name: "overlong feature entry rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{strings.Repeat("f", 65)}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entry is too long"),
		},
		{
			name: "feature entry with control characters rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledFeatures = []string{"feature\x00x"}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entry must not contain control characters"),
		},
		{
			name: "duplicate payment methods rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledPaymentMethods = []string{"ach", "ach"}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entries must be unique"),
		},
		{
			name: "empty payment method rejected",
			settings: func() entity.TenantSettings {
				s := baseSettings
				s.EnabledPaymentMethods = []string{""}
				return s
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "feature entries must be non-empty"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.settings.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseTenantStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		status         string
		expectedResult entity.TenantStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "active status",
			status:         "ACTIVE",
			expectedResult: entity.TenantActive,
			expectedError:  nil,
		},
		{
			name:           "suspended status",
			status:         "SUSPENDED",
			expectedResult: entity.TenantSuspended,
			expectedError:  nil,
		},
		{
			name:           "closed status",
			status:         "CLOSED",
			expectedResult: entity.TenantClosed,
			expectedError:  nil,
		},
		{
			name:           "unknown status",
			status:         "WEIRD",
			expectedResult: entity.TenantStatus(""),
			expectedError:  entity.NewError("TENANT_STATUS_INVALID", "tenant status is invalid"),
		},
		{
			name:           "empty status",
			status:         "",
			expectedResult: entity.TenantStatus(""),
			expectedError:  entity.NewError("TENANT_STATUS_INVALID", "tenant status is invalid"),
		},
		{
			name:           "whitespace status",
			status:         "   ",
			expectedResult: entity.TenantStatus(""),
			expectedError:  entity.NewError("TENANT_STATUS_INVALID", "tenant status is invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := entity.ParseTenantStatus(tc.status)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
