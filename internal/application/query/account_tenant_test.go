package query_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

func TestAccountQueryServiceGetAccount(t *testing.T) {
	t.Parallel()

	stored := entity.AccountData{ID: "a-1", TenantID: "t-1", Version: 1}

	type testCase struct {
		name           string
		query          port.AccountQuery
		programmed     entity.AccountData
		programmedErr  error
		expectedResult port.AccountResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find by id succeeds",
			query: port.AccountQuery{
				TenantID:  "t-1",
				AccountID: "a-1",
			},
			programmed:    stored,
			programmedErr: nil,
			expectedResult: port.AccountResult{
				Account: stored,
			},
			expectedError: nil,
		},
		{
			name: "account not found returns error",
			query: port.AccountQuery{
				TenantID:  "t-1",
				AccountID: "a-missing",
			},
			programmed:     entity.AccountData{},
			programmedErr:  entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
			expectedResult: port.AccountResult{},
			expectedError:  entity.NewError("ACCOUNT_NOT_FOUND", "account is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accounts := mockdomain.NewMockAccountRepository(t)
			accounts.EXPECT().
				FindByID(mock.Anything, tc.query.TenantID, tc.query.AccountID).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewAccountQueryService(query.AccountQueryServiceParams{Accounts: accounts})
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
		query          port.AccountListQuery
		programmed     []entity.AccountData
		programmedNext string
		programmedErr  error
		expectedResult port.AccountPage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "page accounts returns slice and cursor",
			query: port.AccountListQuery{
				TenantID: "t-1",
				Limit:    10,
			},
			programmed:     stored,
			programmedNext: "cur-2",
			programmedErr:  nil,
			expectedResult: port.AccountPage{
				Accounts:   stored,
				NextCursor: "cur-2",
			},
			expectedError: nil,
		},
		{
			name: "list accounts error propagates",
			query: port.AccountListQuery{
				TenantID: "t-1",
				Limit:    10,
			},
			programmed:     nil,
			programmedNext: "",
			programmedErr:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
			expectedResult: port.AccountPage{},
			expectedError:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
		},
		{
			name: "non-positive limit returns validation error",
			query: port.AccountListQuery{
				TenantID: "t-1",
				Limit:    0,
			},
			programmed:     nil,
			programmedNext: "",
			programmedErr:  nil,
			expectedResult: port.AccountPage{},
			expectedError:  entity.NewError("INVALID_PAGE_LIMIT", "limit must be between 1 and 100"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			accounts := mockdomain.NewMockAccountRepository(t)
			if tc.query.Limit > 0 && tc.query.Limit <= 100 {
				accounts.EXPECT().
					FindByTenant(mock.Anything, tc.query.TenantID, tc.query.Cursor, tc.query.Limit).
					Return(tc.programmed, tc.programmedNext, tc.programmedErr).
					Once()
			}

			svc := query.NewAccountQueryService(query.AccountQueryServiceParams{Accounts: accounts})
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
		query          port.TenantQuery
		programmed     entity.TenantData
		programmedErr  error
		expectedResult port.TenantResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find by id succeeds",
			query: port.TenantQuery{
				TenantID: "t-1",
			},
			programmed:    stored,
			programmedErr: nil,
			expectedResult: port.TenantResult{
				Tenant: stored,
			},
			expectedError: nil,
		},
		{
			name: "tenant not found returns error",
			query: port.TenantQuery{
				TenantID: "t-missing",
			},
			programmed:     entity.TenantData{},
			programmedErr:  entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
			expectedResult: port.TenantResult{},
			expectedError:  entity.NewError("TENANT_NOT_FOUND", "tenant is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenants := mockcommand.NewMockTenantStore(t)
			tenants.EXPECT().
				FindByID(mock.Anything, tc.query.TenantID).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewTenantQueryService(query.TenantQueryServiceParams{Tenants: tenants})
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
		programmed     []entity.TenantData
		programmedErr  error
		expectedResult []entity.TenantData
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "list tenants returns all tenants",
			programmed:     stored,
			programmedErr:  nil,
			expectedResult: stored,
			expectedError:  nil,
		},
		{
			name:           "list error propagates",
			programmed:     nil,
			programmedErr:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
			expectedResult: nil,
			expectedError:  entity.NewError("DB_UNAVAILABLE", "database timeout"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenants := mockcommand.NewMockTenantStore(t)
			tenants.EXPECT().
				ListTenants(mock.Anything).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewTenantQueryService(query.TenantQueryServiceParams{Tenants: tenants})
			actualResult, err := svc.ListTenants(context.Background())
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
