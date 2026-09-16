package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestToAccountDTO(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		account        entity.AccountData
		cursor         string
		expectedResult dto.AccountDTO
	}

	testCases := []testCase{
		{
			name: "account maps every field with cursor",
			account: entity.AccountData{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-1", Number: "6000", Name: "operating",
				Class: "LIABILITY", AssetCode: "USD", Status: "ACTIVE", Purpose: "ops",
				Metadata: map[string]string{"k": "v"}, Version: 3,
			},
			cursor: "cursor-3",
			expectedResult: dto.AccountDTO{
				ID: "a-1", TenantID: "t-1", LedgerID: "l-1", Number: "6000", Name: "operating",
				Class: "LIABILITY", AssetCode: "USD", Status: "ACTIVE", Purpose: "ops",
				Metadata: map[string]string{"k": "v"}, Version: 3, Cursor: "cursor-3",
			},
		},
		{
			name: "empty metadata maps without omissions",
			account: entity.AccountData{
				ID: "a-2", TenantID: "t-1", LedgerID: "l-1", Number: "6001", Name: "vault",
				Class: "ASSET", AssetCode: "EUR", Status: "FROZEN", Version: 1,
			},
			cursor: "",
			expectedResult: dto.AccountDTO{
				ID: "a-2", TenantID: "t-1", LedgerID: "l-1", Number: "6001", Name: "vault",
				Class: "ASSET", AssetCode: "EUR", Status: "FROZEN", Version: 1,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToAccountDTO(tc.account, tc.cursor))
		})
	}
}

func TestToTenantDTO(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         entity.TenantData
		cursor         string
		expectedResult dto.TenantDTO
	}

	testCases := []testCase{
		{
			name: "tenant maps identity plus settings",
			tenant: entity.TenantData{
				ID: "t-1", Name: "acme", Region: "us-east", Status: "ACTIVE",
				Settings: entity.TenantSettings{
					DefaultCurrency:       "USD",
					Timezone:              "UTC",
					EnabledFeatures:       []string{"transfers"},
					EnabledPaymentMethods: []string{"ACH"},
				},
				Version: 2,
			},
			cursor: "cursor-9",
			expectedResult: dto.TenantDTO{
				ID: "t-1", Name: "acme", Region: "us-east", Status: "ACTIVE",
				Settings: dto.TenantSettingsDTO{
					DefaultCurrency:       "USD",
					Timezone:              "UTC",
					EnabledFeatures:       []string{"transfers"},
					EnabledPaymentMethods: []string{"ACH"},
				},
				Version: 2, Cursor: "cursor-9",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToTenantDTO(tc.tenant, tc.cursor))
		})
	}
}
