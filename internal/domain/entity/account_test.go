package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testMain     = "Main"
	testLedgerID = "20000000-0000-4000-8000-000000000001"
	testTenantID = "10000000-0000-4000-8000-000000000001"
	testUSD      = "USD"
	testAccount1 = "30000000-0000-4000-8000-000000000001"
	testAccount2 = "30000000-0000-4000-8000-000000000002"
	testPosting1 = "40000000-0000-4000-8000-000000000001"
	testPeriod1  = "50000000-0000-4000-8000-000000000001"
	testJournal1 = "60000000-0000-4000-8000-000000000001"
)

func TestDomainError(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		err             *entity.Error
		expectedCode    string
		expectedMessage string
		expectedString  string
	}

	testCases := []testCase{
		{
			name:            "NewError formatted string",
			err:             entity.NewError("ACCOUNT_FROZEN", "account is frozen"),
			expectedCode:    "ACCOUNT_FROZEN",
			expectedMessage: "account is frozen",
			expectedString:  "ACCOUNT_FROZEN: account is frozen",
		},
		{
			name:            "Errorf formatted message",
			err:             entity.Errorf("E_X", "code %d", 1),
			expectedCode:    "E_X",
			expectedMessage: "code 1",
			expectedString:  "E_X: code 1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedCode, tc.err.Code)
			assert.Equal(t, tc.expectedMessage, tc.err.Message)
			assert.Equal(t, tc.expectedString, tc.err.Error())
		})
	}
}

func TestNewLedger(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		id            valueobject.LedgerID
		tenantID      valueobject.TenantID
		nameField     string
		assetCode     valueobject.AssetCode
		chartVersion  string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid ledger initialization",
			id:            testLedgerID,
			tenantID:      testTenantID,
			nameField:     testMain,
			assetCode:     testUSD,
			chartVersion:  "v1",
			expectedError: nil,
		},
		{
			name:          "empty id permitted before persistence",
			id:            "",
			tenantID:      testTenantID,
			nameField:     testMain,
			assetCode:     testUSD,
			chartVersion:  "v1",
			expectedError: nil,
		},
		{
			name:          "invalid id format",
			id:            "not-a-uuid",
			tenantID:      testTenantID,
			nameField:     testMain,
			assetCode:     testUSD,
			chartVersion:  "v1",
			expectedError: entity.NewError("LEDGER_ID_INVALID", "ledger id is invalid"),
		},
		{
			name:          "empty tenant",
			id:            testLedgerID,
			tenantID:      "",
			nameField:     testMain,
			assetCode:     testUSD,
			chartVersion:  "v1",
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name:          "empty name",
			id:            testLedgerID,
			tenantID:      testTenantID,
			nameField:     "",
			assetCode:     testUSD,
			chartVersion:  "v1",
			expectedError: entity.NewError("LEDGER_NAME_REQUIRED", "ledger name is required"),
		},
		{
			name:          "empty asset",
			id:            testLedgerID,
			tenantID:      testTenantID,
			nameField:     testMain,
			assetCode:     "",
			chartVersion:  "v1",
			expectedError: entity.NewError("LEDGER_ASSET_REQUIRED", "base asset code is required"),
		},
		{
			name:          "empty chart version",
			id:            testLedgerID,
			tenantID:      testTenantID,
			nameField:     testMain,
			assetCode:     testUSD,
			chartVersion:  "",
			expectedError: entity.NewError("LEDGER_CHART_REQUIRED", "chart version is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ledger, err := entity.NewLedger(tc.id, tc.tenantID, tc.nameField, tc.assetCode, tc.chartVersion)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.id, ledger.ID)
				assert.Equal(t, tc.tenantID, ledger.TenantID)
			}
		})
	}
}

func TestAccountDataValidate(t *testing.T) {
	t.Parallel()

	valid := entity.AccountData{
		ID:        testAccount1,
		TenantID:  testTenantID,
		LedgerID:  testLedgerID,
		Number:    "1000",
		Name:      "Cash",
		Class:     valueobject.ClassAsset,
		AssetCode: testUSD,
		Status:    valueobject.StatusActive,
		Version:   1,
	}

	type testCase struct {
		name          string
		account       entity.AccountData
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid active account",
			account:       valid,
			expectedError: nil,
		},
		{
			name: "missing id permitted before persistence",
			account: func() entity.AccountData {
				a := valid
				a.ID = ""
				return a
			}(),
			expectedError: nil,
		},
		{
			name: "invalid id format",
			account: func() entity.AccountData {
				a := valid
				a.ID = "not-a-uuid"
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_ID_INVALID", "account id is invalid"),
		},
		{
			name: "missing tenant id",
			account: func() entity.AccountData {
				a := valid
				a.TenantID = ""
				return a
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger id",
			account: func() entity.AccountData {
				a := valid
				a.LedgerID = ""
				return a
			}(),
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "missing number",
			account: func() entity.AccountData {
				a := valid
				a.Number = ""
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_NUMBER_REQUIRED", "account number is required"),
		},
		{
			name: "missing name",
			account: func() entity.AccountData {
				a := valid
				a.Name = ""
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_NAME_REQUIRED", "account name is required"),
		},
		{
			name: "invalid class",
			account: func() entity.AccountData {
				a := valid
				a.Class = "NOPE"
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_CLASS_INVALID", "account class is invalid"),
		},
		{
			name: "missing asset code",
			account: func() entity.AccountData {
				a := valid
				a.AssetCode = ""
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_ASSET_REQUIRED", "asset code is explicit on every postable account"),
		},
		{
			name: "invalid status",
			account: func() entity.AccountData {
				a := valid
				a.Status = "PENDING"
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_STATUS_INVALID", "account status is invalid"),
		},
		{
			name: "zero version",
			account: func() entity.AccountData {
				a := valid
				a.Version = 0
				return a
			}(),
			expectedError: entity.NewError("ACCOUNT_VERSION_INVALID", "version starts at 1"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.account.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
