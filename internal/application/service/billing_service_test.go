package service_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/service"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

var billAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type usageStoreFake struct {
	mu   sync.Mutex
	seen map[string]bool
	bins map[string]int64
}

func usageKey(tenant, day string, kind service.UsageKind) string {
	return tenant + "|" + day + "|" + string(kind)
}

func (s *usageStoreFake) AppendEvent(_ context.Context, event service.UsageEvent) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen == nil {
		s.seen = map[string]bool{}
		s.bins = map[string]int64{}
	}
	if s.seen[event.EventID] {
		return false, nil
	}
	s.seen[event.EventID] = true
	day := event.OccurredAt.UTC().Format("2006-01-02")
	s.bins[usageKey(event.TenantID, day, event.Kind)] += event.Quantity
	return true, nil
}

func (s *usageStoreFake) DayTotals(_ context.Context, tenantID, day string) (map[service.UsageKind]int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	totals := map[service.UsageKind]int64{}
	for _, kind := range []service.UsageKind{service.UsageTransactionsPosted, service.UsageAPICalls, service.UsagePayouts, service.UsageReportsGenerated} {
		totals[kind] = s.bins[usageKey(tenantID, day, kind)]
	}
	return totals, nil
}

func (s *usageStoreFake) PeriodLines(_ context.Context, tenantID, fromDay, toDay string) ([]service.DailyUsage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []service.DailyUsage
	for key, quantity := range s.bins {
		parts := strings.SplitN(key, "|", 3)
		if len(parts) != 3 {
			continue
		}
		tenant, day, kind := parts[0], parts[1], parts[2]
		if tenant != tenantID || day < fromDay || day > toDay || quantity == 0 {
			continue
		}
		out = append(out, service.DailyUsage{TenantID: tenant, Day: day, Kind: service.UsageKind(kind), Quantity: quantity})
	}
	return out, nil
}

func TestBillingRecord(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		event           service.UsageEvent
		replays         int
		expectedApplied bool
		expectedError   error
		expectedTotal   int64
	}

	testCases := []testCase{
		{
			name: "first record applies",
			event: service.UsageEvent{
				EventID: "e-1", TenantID: "t-1", Kind: service.UsageTransactionsPosted,
				Quantity: 3, OccurredAt: billAt,
			},
			replays:         0,
			expectedApplied: true,
			expectedError:   nil,
			expectedTotal:   3,
		},
		{
			name: "replay does not double count",
			event: service.UsageEvent{
				EventID: "e-1", TenantID: "t-1", Kind: service.UsageTransactionsPosted,
				Quantity: 3, OccurredAt: billAt,
			},
			replays:         2,
			expectedApplied: false,
			expectedError:   nil,
			expectedTotal:   3,
		},
		{
			name: "unknown kind rejected",
			event: service.UsageEvent{
				EventID: "e-9", TenantID: "t-1", Kind: "telepathy",
				Quantity: 1, OccurredAt: billAt,
			},
			replays:         0,
			expectedApplied: false,
			expectedError:   entity.NewError("USAGE_KIND_UNKNOWN", "usage kind is unknown"),
			expectedTotal:   0,
		},
		{
			name: "zero quantity rejected",
			event: service.UsageEvent{
				EventID: "e-9", TenantID: "t-1", Kind: service.UsageAPICalls,
				Quantity: 0, OccurredAt: billAt,
			},
			replays:         0,
			expectedApplied: false,
			expectedError:   entity.NewError("USAGE_QUANTITY_INVALID", "usage quantity must be positive"),
			expectedTotal:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})
			var applied bool
			var err error
			for i := 0; i <= tc.replays; i++ {
				applied, err = svc.Record(context.Background(), tc.event)
			}
			assert.Equal(t, tc.expectedApplied, applied)
			assert.Equal(t, tc.expectedError, err)
			totals, totalsErr := svc.DayTotals(context.Background(), "t-1", "2026-09-15")
			require.NoError(t, totalsErr)
			assert.Equal(t, tc.expectedTotal, totals[tc.event.Kind])
		})
	}
}

func TestBillingExport(t *testing.T) {
	t.Parallel()

	t.Run("export matches hand computation", func(t *testing.T) {
		svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})
		events := []service.UsageEvent{
			{EventID: "e-1", TenantID: "t-1", Kind: service.UsageTransactionsPosted, Quantity: 10, OccurredAt: billAt},
			{EventID: "e-2", TenantID: "t-1", Kind: service.UsageAPICalls, Quantity: 100, OccurredAt: billAt},
			{EventID: "e-3", TenantID: "t-1", Kind: service.UsageTransactionsPosted, Quantity: 5, OccurredAt: billAt.Add(24 * time.Hour)},
			{EventID: "e-4", TenantID: "t-2", Kind: service.UsagePayouts, Quantity: 7, OccurredAt: billAt},
		}
		for _, event := range events {
			applied, err := svc.Record(context.Background(), event)
			require.NoError(t, err)
			require.True(t, applied)
		}
		exported, err := svc.ExportCSV(context.Background(), "t-1", "2026-09-15", "2026-09-16")
		require.NoError(t, err)
		assert.Equal(t, "tenant,day,kind,quantity\n"+
			"t-1,2026-09-15,api-calls,100\n"+
			"t-1,2026-09-15,transactions-posted,10\n"+
			"t-1,2026-09-16,transactions-posted,5\n"+
			"t-1,2026-09-16,TOTAL,115\n", string(exported))
	})

	t.Run("inverted range rejected", func(t *testing.T) {
		svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})
		_, err := svc.ExportCSV(context.Background(), "t-1", "2026-09-16", "2026-09-15")
		assert.Equal(t, entity.NewError("BILLING_RANGE_INVALID", "to day must not precede from day"), err)
	})

	t.Run("bad day rejected", func(t *testing.T) {
		svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})
		_, err := svc.ExportCSV(context.Background(), "t-1", "15-09-2026", "2026-09-16")
		assert.Equal(t, entity.NewError("BILLING_DAY_INVALID", "from day must be YYYY-MM-DD"), err)
	})

	t.Run("day totals validation branches", func(t *testing.T) {
		svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})

		_, err := svc.DayTotals(context.Background(), "", "2026-09-15")
		assert.Equal(t, entity.NewError("TENANT_ID_REQUIRED", "tenant id is required"), err)

		_, err = svc.DayTotals(context.Background(), "t-1", "yesterday")
		assert.Equal(t, entity.NewError("BILLING_DAY_INVALID", "day must be YYYY-MM-DD"), err)

		totals, err := svc.DayTotals(context.Background(), "t-1", "2026-09-15")
		assert.NoError(t, err)
		assert.Equal(t, int64(0), totals[service.UsagePayouts])
	})

	t.Run("record validation branches", func(t *testing.T) {
		svc := service.NewBillingService(service.BillingServiceParams{Store: &usageStoreFake{}})

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
