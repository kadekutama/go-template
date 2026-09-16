package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestAccountQueryServiceGetAccount(t *testing.T) {
	t.Parallel()

	stored := entity.AccountData{ID: "a-1", TenantID: "t-1", Version: 1}

	type testCase struct {
		name           string
		accounts       *stubAccounts
		query          port.AccountQuery
		expectedResult port.AccountResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find by id succeeds",
			accounts: &stubAccounts{
				account: stored,
			},
			query: port.AccountQuery{
				TenantID:  "t-1",
				AccountID: "a-1",
			},
			expectedResult: port.AccountResult{
				Account: stored,
			},
			expectedError: nil,
		},
		{
			name: "account not found returns error",
			accounts: &stubAccounts{
				err: entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
			},
			query: port.AccountQuery{
				TenantID:  "t-1",
				AccountID: "a-missing",
			},
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewAccountQueryService(query.AccountQueryServiceParams{Accounts: tc.accounts})
			actualResult, err := svc.GetAccount(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestAccountQueryServiceListAccounts(t *testing.T) {
	t.Parallel()

	stored := []entity.AccountData{
		{ID: "a-1", TenantID: "t-1", Version: 1},
		{ID: "a-2", TenantID: "t-1", Version: 1},
	}

	type testCase struct {
		name           string
		accounts       *stubAccounts
		query          port.AccountListQuery
		expectedResult port.AccountPage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "page accounts returns slice and cursor",
			accounts: &stubAccounts{
				accounts: stored,
				next:     "cur-2",
			},
			query: port.AccountListQuery{
				TenantID: "t-1",
				Limit:    10,
			},
			expectedResult: port.AccountPage{
				Accounts:   stored,
				NextCursor: "cur-2",
			},
			expectedError: nil,
		},
		{
			name: "list accounts error propagates",
			accounts: &stubAccounts{
				err: entity.NewError("DB_UNAVAILABLE", "database timeout"),
			},
			query: port.AccountListQuery{
				TenantID: "t-1",
			},
			expectedResult: port.AccountPage{},
			expectedError:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewAccountQueryService(query.AccountQueryServiceParams{Accounts: tc.accounts})
			actualResult, err := svc.ListAccounts(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenantQueryServiceGetTenant(t *testing.T) {
	t.Parallel()

	stored := entity.TenantData{ID: "t-1", Name: "acme", Version: 1}

	type testCase struct {
		name           string
		tenants        *stubTenants
		query          port.TenantQuery
		expectedResult port.TenantResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find by id succeeds",
			tenants: &stubTenants{
				tenant: stored,
			},
			query: port.TenantQuery{
				TenantID: "t-1",
			},
			expectedResult: port.TenantResult{
				Tenant: stored,
			},
			expectedError: nil,
		},
		{
			name: "tenant not found returns error",
			tenants: &stubTenants{
				err: entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
			},
			query: port.TenantQuery{
				TenantID: "t-missing",
			},
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewTenantQueryService(query.TenantQueryServiceParams{Tenants: tc.tenants})
			actualResult, err := svc.GetTenant(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTenantQueryServiceListTenants(t *testing.T) {
	t.Parallel()

	stored := []entity.TenantData{
		{ID: "t-1", Name: "acme-1", Version: 1},
		{ID: "t-2", Name: "acme-2", Version: 1},
	}

	type testCase struct {
		name           string
		tenants        *stubTenants
		expectedResult []entity.TenantData
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "list tenants returns all tenants",
			tenants: &stubTenants{
				tenants: stored,
			},
			expectedResult: stored,
			expectedError:  nil,
		},
		{
			name: "list error propagates",
			tenants: &stubTenants{
				err: entity.NewError("DB_UNAVAILABLE", "database timeout"),
			},
			expectedResult: nil,
			expectedError:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewTenantQueryService(query.TenantQueryServiceParams{Tenants: tc.tenants})
			actualResult, err := svc.ListTenants(context.Background())
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
