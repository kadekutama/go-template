package rls_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/rls"
)

func TestWithTenantRejectsBlank(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		tenant        valueobject.TenantID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "blank tenant rejected",
			ctx:           context.Background(),
			tenant:        "",
			expectedError: errors.New("rls: tenant is required"),
		},
		{
			name:          "tenant stored",
			ctx:           context.Background(),
			tenant:        "10000000-0000-4000-8000-000000000001",
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, err := rls.WithTenant(tc.ctx, tc.tenant)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
				assert.Nil(t, ctx)
			} else {
				require.NoError(t, err)
				require.NotNil(t, ctx)

				resolved, resolveErr := rls.TenantFromContext(ctx)
				require.NoError(t, resolveErr)
				assert.Equal(t, tc.tenant, resolved)
			}
		})
	}
}

func TestTenantFromContextFailsClosed(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		expectedResult valueobject.TenantID
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "bare context fails",
			ctx:            context.Background(),
			expectedResult: "",
			expectedError:  errors.New("rls: tenant missing from context"),
		},
		{
			name: "stored tenant resolves",
			ctx: func() context.Context {
				c, err := rls.WithTenant(context.Background(), "10000000-0000-4000-8000-000000000001")
				if err != nil {
					panic(err)
				}
				return c
			}(),
			expectedResult: "10000000-0000-4000-8000-000000000001",
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := rls.TenantFromContext(tc.ctx)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestResolveTenantAgreement(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		arg            valueobject.TenantID
		expectedResult valueobject.TenantID
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "context only resolves",
			ctx: func() context.Context {
				c, err := rls.WithTenant(context.Background(), "10000000-0000-4000-8000-000000000001")
				if err != nil {
					panic(err)
				}
				return c
			}(),
			arg:            "",
			expectedResult: "10000000-0000-4000-8000-000000000001",
			expectedError:  nil,
		},
		{
			name:           "argument only resolves",
			ctx:            context.Background(),
			arg:            "10000000-0000-4000-8000-000000000002",
			expectedResult: "10000000-0000-4000-8000-000000000002",
			expectedError:  nil,
		},
		{
			name: "agreement resolves",
			ctx: func() context.Context {
				c, err := rls.WithTenant(context.Background(), "10000000-0000-4000-8000-000000000001")
				if err != nil {
					panic(err)
				}
				return c
			}(),
			arg:            "10000000-0000-4000-8000-000000000001",
			expectedResult: "10000000-0000-4000-8000-000000000001",
			expectedError:  nil,
		},
		{
			name: "disagreement fails closed",
			ctx: func() context.Context {
				c, err := rls.WithTenant(context.Background(), "10000000-0000-4000-8000-000000000001")
				if err != nil {
					panic(err)
				}
				return c
			}(),
			arg:            "tnt-test-09",
			expectedResult: "",
			expectedError:  errors.New("rls: context tenant and argument tenant disagree"),
		},
		{
			name:           "both absent fails",
			ctx:            context.Background(),
			arg:            "",
			expectedResult: "",
			expectedError:  errors.New("rls: tenant is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := rls.ResolveTenant(tc.ctx, tc.arg)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestApplyTenantGuards(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		db            *gorm.DB
		tenant        valueobject.TenantID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "blank tenant never reaches SQL",
			ctx:           context.Background(),
			db:            nil,
			tenant:        "",
			expectedError: errors.New("rls: tenant is required"),
		},
		{
			name:          "nil DB fails before SQL",
			ctx:           context.Background(),
			db:            nil,
			tenant:        "10000000-0000-4000-8000-000000000001",
			expectedError: errors.New("rls: DB is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := rls.ApplyTenant(tc.ctx, tc.db, tc.tenant)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}
