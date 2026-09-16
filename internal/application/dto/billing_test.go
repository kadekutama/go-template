package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/application/service"
)

func TestToBillingExport(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		lines          []service.DailyUsage
		expectedResult dto.BillingExportDTO
	}

	testCases := []testCase{
		{
			name: "lines sum to total",
			lines: []service.DailyUsage{
				{TenantID: "t-1", Day: "2026-09-15", Kind: service.UsageTransactionsPosted, Quantity: 10},
				{TenantID: "t-1", Day: "2026-09-15", Kind: service.UsageAPICalls, Quantity: 100},
			},
			expectedResult: dto.BillingExportDTO{
				Lines: []dto.UsageLineDTO{
					{Tenant: "t-1", Day: "2026-09-15", Kind: "transactions-posted", Quantity: 10},
					{Tenant: "t-1", Day: "2026-09-15", Kind: "api-calls", Quantity: 100},
				},
				Total: 110,
			},
		},
		{
			name:  "empty lines total zero",
			lines: nil,
			expectedResult: dto.BillingExportDTO{
				Lines: []dto.UsageLineDTO{},
				Total: 0,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToBillingExport(tc.lines))
		})
	}
}
