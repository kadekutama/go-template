package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/service"
	"github.com/kadekutama/go-template/internal/domain/entity"
	mockservice "github.com/kadekutama/go-template/test/mock/service"
)

var billAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func TestBillingRecord(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		event           service.UsageEvent
		mockSetup       func(m *mockservice.MockUsageStore)
		expectedApplied bool
		expectedError   error
	}

	testCases := []testCase{
		{
			name: "first record applies",
			event: service.UsageEvent{
				EventID: "e-1", TenantID: "t-1", Kind: service.UsageTransactionsPosted,
				Quantity: 3, OccurredAt: billAt,
			},
			mockSetup: func(m *mockservice.MockUsageStore) {
				m.EXPECT().AppendEvent(mock.Anything, mock.MatchedBy(func(e service.UsageEvent) bool {
					return e.EventID == "e-1"
				})).Return(true, nil).Once()
			},
			expectedApplied: true,
			expectedError:   nil,
		},
		{
			name: "replay does not double count",
			event: service.UsageEvent{
				EventID: "e-1", TenantID: "t-1", Kind: service.UsageTransactionsPosted,
				Quantity: 3, OccurredAt: billAt,
			},
			mockSetup: func(m *mockservice.MockUsageStore) {
				m.EXPECT().AppendEvent(mock.Anything, mock.MatchedBy(func(e service.UsageEvent) bool {
					return e.EventID == "e-1"
				})).Return(false, nil).Once()
			},
			expectedApplied: false,
			expectedError:   nil,
		},
		{
			name: "unknown kind rejected",
			event: service.UsageEvent{
				EventID: "e-9", TenantID: "t-1", Kind: "telepathy",
				Quantity: 1, OccurredAt: billAt,
			},
			mockSetup:       func(_ *mockservice.MockUsageStore) {},
			expectedApplied: false,
			expectedError:   entity.NewError("USAGE_KIND_UNKNOWN", "usage kind is unknown"),
		},
		{
			name: "zero quantity rejected",
			event: service.UsageEvent{
				EventID: "e-9", TenantID: "t-1", Kind: service.UsageAPICalls,
				Quantity: 0, OccurredAt: billAt,
			},
			mockSetup:       func(_ *mockservice.MockUsageStore) {},
			expectedApplied: false,
			expectedError:   entity.NewError("USAGE_QUANTITY_INVALID", "usage quantity must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := mockservice.NewMockUsageStore(t)
			tc.mockSetup(store)
			svc := service.NewBillingService(service.BillingServiceParams{Store: store})
			applied, err := svc.Record(context.Background(), tc.event)
			assert.Equal(t, tc.expectedApplied, applied)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBillingExport(t *testing.T) {
	t.Parallel()

	t.Run("export matches hand computation", func(t *testing.T) {
		store := mockservice.NewMockUsageStore(t)
		store.EXPECT().PeriodLines(mock.Anything, "t-1", "2026-09-15", "2026-09-16").Return([]service.DailyUsage{
			{TenantID: "t-1", Day: "2026-09-15", Kind: service.UsageAPICalls, Quantity: 100},
			{TenantID: "t-1", Day: "2026-09-15", Kind: service.UsageTransactionsPosted, Quantity: 10},
			{TenantID: "t-1", Day: "2026-09-16", Kind: service.UsageTransactionsPosted, Quantity: 5},
		}, nil).Once()

		svc := service.NewBillingService(service.BillingServiceParams{Store: store})
		exported, err := svc.ExportCSV(context.Background(), "t-1", "2026-09-15", "2026-09-16")
		require.NoError(t, err)
		assert.Equal(t, "tenant,day,kind,quantity\n"+
			"t-1,2026-09-15,api-calls,100\n"+
			"t-1,2026-09-15,transactions-posted,10\n"+
			"t-1,2026-09-16,transactions-posted,5\n"+
			"t-1,2026-09-16,TOTAL,115\n", string(exported))
	})

	t.Run("inverted range rejected", func(t *testing.T) {
		store := mockservice.NewMockUsageStore(t)
		svc := service.NewBillingService(service.BillingServiceParams{Store: store})
		_, err := svc.ExportCSV(context.Background(), "t-1", "2026-09-16", "2026-09-15")
		assert.Equal(t, entity.NewError("BILLING_RANGE_INVALID", "to day must not precede from day"), err)
	})

	t.Run("bad day rejected", func(t *testing.T) {
		store := mockservice.NewMockUsageStore(t)
		svc := service.NewBillingService(service.BillingServiceParams{Store: store})
		_, err := svc.ExportCSV(context.Background(), "t-1", "15-09-2026", "2026-09-16")
		assert.Equal(t, entity.NewError("BILLING_DAY_INVALID", "from day must be YYYY-MM-DD"), err)
	})

	t.Run("day totals validation branches", func(t *testing.T) {
		store := mockservice.NewMockUsageStore(t)
		svc := service.NewBillingService(service.BillingServiceParams{Store: store})

		_, err := svc.DayTotals(context.Background(), "", "2026-09-15")
		assert.Equal(t, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"), err)

		_, err = svc.DayTotals(context.Background(), "t-1", "yesterday")
		assert.Equal(t, entity.NewError("BILLING_DAY_INVALID", "day must be YYYY-MM-DD"), err)

		store.EXPECT().DayTotals(mock.Anything, "t-1", "2026-09-15").Return(map[service.UsageKind]int64{
			service.UsagePayouts: 0,
		}, nil).Once()

		totals, err := svc.DayTotals(context.Background(), "t-1", "2026-09-15")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), totals[service.UsagePayouts])
	})

	t.Run("record validation branches", func(t *testing.T) {
		store := mockservice.NewMockUsageStore(t)
		svc := service.NewBillingService(service.BillingServiceParams{Store: store})

		_, err := svc.Record(context.Background(), service.UsageEvent{
			TenantID: "t-1", Kind: service.UsageAPICalls, Quantity: 1, OccurredAt: billAt,
		})
		assert.Equal(t, entity.NewError("USAGE_EVENT_REQUIRED", "usage event id is required"), err)

		_, err = svc.Record(context.Background(), service.UsageEvent{
			EventID: "e-1", Kind: service.UsageAPICalls, Quantity: 1, OccurredAt: billAt,
		})
		assert.Equal(t, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"), err)

		_, err = svc.Record(context.Background(), service.UsageEvent{
			EventID: "e-1", TenantID: "t-1", Kind: service.UsageAPICalls, Quantity: 1,
		})
		assert.Equal(t, entity.NewError("USAGE_TIME_REQUIRED", "usage time is required"), err)
	})
}
