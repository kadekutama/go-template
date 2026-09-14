package specification_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func activeAccount() entity.AccountData {
	return entity.AccountData{
		ID:        testAccount1,
		TenantID:  testTenant1,
		LedgerID:  testLedger1,
		Number:    "1000",
		Name:      "Cash",
		Class:     valueobject.ClassAsset,
		AssetCode: testUSD,
		Status:    valueobject.StatusActive,
		Version:   1,
	}
}

func TestAccountActiveMatrix(t *testing.T) {
	t.Parallel()

	base := activeAccount()

	type testCase struct {
		name                  string
		ctx                   context.Context
		account               entity.AccountData
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name:                  "active account passes",
			ctx:                   context.Background(),
			account:               base,
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "frozen account rejected",
			ctx:  context.Background(),
			account: func() entity.AccountData {
				a := base
				a.Status = valueobject.StatusFrozen
				return a
			}(),
			expectedPassed:        false,
			expectedViolationCode: "ACCOUNT_FROZEN",
		},
		{
			name: "closed account rejected",
			ctx:  context.Background(),
			account: func() entity.AccountData {
				a := base
				a.Status = valueobject.StatusClosed
				return a
			}(),
			expectedPassed:        false,
			expectedViolationCode: "ACCOUNT_CLOSED",
		},
		{
			name: "unknown status rejected",
			ctx:  context.Background(),
			account: func() entity.AccountData {
				a := base
				a.Status = "WEIRD"
				return a
			}(),
			expectedPassed:        false,
			expectedViolationCode: "ACCOUNT_INACTIVE",
		},
		{
			name:                  "zero account rejected without panic",
			ctx:                   context.Background(),
			account:               entity.AccountData{},
			expectedPassed:        false,
			expectedViolationCode: "ACCOUNT_INACTIVE",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.AccountActive().Evaluate(tc.ctx, tc.account)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}

func TestSameTenant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                  string
		ctx                   context.Context
		pair                  specification.TenantPair
		expectedPassed        bool
		expectedViolationCode string
	}

	testCases := []testCase{
		{
			name: "same tenant passes",
			ctx:  context.Background(),
			pair: specification.TenantPair{
				A: testTenant1,
				B: testTenant1,
			},
			expectedPassed:        true,
			expectedViolationCode: "",
		},
		{
			name: "different tenants rejected",
			ctx:  context.Background(),
			pair: specification.TenantPair{
				A: testTenant1,
				B: "t-2",
			},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_MISMATCH",
		},
		{
			name:                  "empty tenants rejected",
			ctx:                   context.Background(),
			pair:                  specification.TenantPair{},
			expectedPassed:        false,
			expectedViolationCode: "TENANT_MISMATCH",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			res := specification.SameTenant().Evaluate(tc.ctx, tc.pair)
			assert.Equal(t, tc.expectedPassed, res.Passed())
			if tc.expectedViolationCode != "" {
				assert.NotEmpty(t, res.Violations)
				assert.Equal(t, tc.expectedViolationCode, res.Violations[0].Code)
			}
		})
	}
}
