package command_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

type subscriptionStoreFake struct {
	mu   sync.Mutex
	subs map[string]command.Subscription
}

func (s *subscriptionStoreFake) CreateSubscription(_ context.Context, sub command.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.subs == nil {
		s.subs = map[string]command.Subscription{}
	}
	s.subs[sub.ID] = sub
	return nil
}

func (s *subscriptionStoreFake) FindSubscription(_ context.Context, _ valueobject.TenantID, id string) (command.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub, ok := s.subs[id]
	if !ok {
		return command.Subscription{}, entity.NewError("SUBSCRIPTION_NOT_FOUND", "subscription is unknown")
	}
	return sub, nil
}

func (s *subscriptionStoreFake) UpdateSubscription(_ context.Context, sub command.Subscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[sub.ID] = sub
	return nil
}

func (s *subscriptionStoreFake) DeleteSubscription(_ context.Context, _ valueobject.TenantID, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
	return nil
}

func (s *subscriptionStoreFake) ListDueSubscriptions(_ context.Context, _ valueobject.TenantID, now time.Time) ([]command.Subscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.Subscription
	for _, sub := range s.subs {
		if !sub.LastRun.After(now.Add(-24 * time.Hour)) {
			out = append(out, sub)
		}
	}
	return out, nil
}

func newSubscriptionService(uow *tfrUOW, subs *subscriptionStoreFake, authz *tfrAuthz) *command.SubscriptionService {
	storage := &objectStoreFake{}
	return command.NewSubscriptionService(command.SubscriptionServiceParams{
		UoW:           uow,
		Subscriptions: subs,
		Reports: command.NewReportService(command.ReportServiceParams{
			UoW:     uow,
			Reports: &reportStoreFake{},
			Storage: storage,
			Clock:   tfrClock{},
			IDs:     &tfrIDs{next: tfrTestIDs(40)},
			Authz:   authz,
		}),
		Clock: tfrClock{},
		IDs:   &tfrIDs{next: tfrTestIDs(40)},
		Authz: authz,
	})
}

func TestSubscriptionSubscribe(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		tenant        valueobject.TenantID
		template      string
		cron          string
		destination   string
		actor         string
		key           string
		preload       func(uow *tfrUOW)
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid subscribe succeeds",
			tenant:        tfrTenant,
			template:      "settlement",
			cron:          "0 9 * * *",
			destination:   "webhook:ops",
			actor:         "u-1",
			key:           "key-sub-1",
			preload:       func(_ *tfrUOW) {},
			expectedError: nil,
		},
		{
			name:        "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			tenant:      tfrTenant,
			template:    "settlement",
			cron:        "0 9 * * *",
			destination: "webhook:ops",
			actor:       "u-1",
			key:         "key-sub-1",
			preload: func(uow *tfrUOW) {
				fp := command.Fingerprint("key-sub-1", string(tfrTenant), "settlement", "0 9 * * *", "webhook:ops")
				uow.idem = map[string]tfrIdemEntry{
					"key-sub-1": {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			expectedError: entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name:          "unknown template rejected",
			tenant:        tfrTenant,
			template:      "fortune-cookie",
			cron:          "0 9 * * *",
			destination:   "webhook:ops",
			actor:         "u-1",
			key:           "key-sub-1",
			preload:       func(_ *tfrUOW) {},
			expectedError: entity.NewError("REPORT_TEMPLATE_UNKNOWN", "report template is unknown"),
		},
		{
			name:          "missing tenant rejected",
			tenant:        "",
			template:      "settlement",
			cron:          "0 9 * * *",
			destination:   "webhook:ops",
			actor:         "u-1",
			key:           "key-sub-1",
			preload:       func(_ *tfrUOW) {},
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			subs := &subscriptionStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			tc.preload(uow)
			svc := newSubscriptionService(uow, subs, authz)

			actualResult, err := svc.Subscribe(context.Background(), tc.tenant, tc.template, tc.cron, tc.destination, tc.actor, tc.key)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.NotEmpty(t, actualResult.ID)
				assert.Equal(t, tc.template, actualResult.Template)

				// Idempotency replay
				replayResult, replayErr := svc.Subscribe(context.Background(), tc.tenant, tc.template, tc.cron, tc.destination, tc.actor, tc.key)
				assert.NoError(t, replayErr)
				assert.Equal(t, actualResult.ID, replayResult.ID)
			}
		})
	}
}

func TestSubscriptions(t *testing.T) {
	t.Parallel()

	t.Run("deliver due subscription and unsubscribe", func(t *testing.T) {
		uow := &tfrUOW{}
		subs := &subscriptionStoreFake{}
		authz := &tfrAuthz{denied: map[string]bool{}}
		svc := newSubscriptionService(uow, subs, authz)

		sub, err := svc.Subscribe(context.Background(), tfrTenant, "transaction-volume", "0 9 * * *", "webhook:ops", "u-1", "key-sub-1")
		require.NoError(t, err)

		due, err := svc.DueSubscriptions(context.Background(), tfrTenant, opsAt)
		require.NoError(t, err)
		require.Len(t, due, 1)

		delivered, err := svc.DeliverSubscription(context.Background(), tfrTenant, sub.ID, "run-2026-09-15", "u-1", map[string]string{"period": "2026-09"}, opsAt)
		require.NoError(t, err)
		assert.Equal(t, command.ReportReady, delivered.Status)
		assert.Contains(t, outboxTypes(uow), command.EventReportGenerated)

		updated, err := subs.FindSubscription(context.Background(), tfrTenant, sub.ID)
		require.NoError(t, err)
		assert.Equal(t, opsAt, updated.LastRun)

		require.NoError(t, svc.Unsubscribe(context.Background(), tfrTenant, sub.ID, "u-1"))
		_, err = subs.FindSubscription(context.Background(), tfrTenant, sub.ID)
		assert.Equal(t, entity.NewError("SUBSCRIPTION_NOT_FOUND", "subscription is unknown"), err)
	})
}

func TestRegulatoryAllTypes(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		reportType string
		fields     map[string]string
	}

	testCases := []testCase{
		{
			name:       "call report validates",
			reportType: "CALL_REPORT",
			fields: map[string]string{
				"tenant_id": "t-1", "period_start": "2026-09-01", "period_end": "2026-09-30",
				"total_assets_minor": "100000", "total_liabilities_minor": "90000", "rule_version": "v1",
			},
		},
		{
			name:       "1099 validates",
			reportType: "FORM_1099",
			fields: map[string]string{
				"tenant_id": "t-1", "tax_year": "2026", "payer_tin_hash": "h1",
				"payee_count": "3", "gross_amount_minor": "50000", "rule_version": "v1",
			},
		},
		{
			name:       "fatca validates",
			reportType: "FATCA",
			fields: map[string]string{
				"tenant_id": "t-1", "reporting_period": "2026-09", "account_holder_hash": "h2",
				"account_balance_minor": "10000", "jurisdiction": "US", "rule_version": "v1",
			},
		},
		{
			name:       "crs validates",
			reportType: "CRS",
			fields: map[string]string{
				"tenant_id": "t-1", "reporting_period": "2026-09", "account_holder_hash": "h3",
				"account_balance_minor": "20000", "residence_jurisdiction": "DE", "rule_version": "v1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			store := &complianceStoreFake{}
			storage := &objectStoreFake{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			svc := command.NewComplianceService(command.ComplianceServiceParams{
				UoW: uow, Reviews: store, Storage: storage, Clock: tfrClock{},
				IDs: &tfrIDs{next: tfrTestIDs(40)}, Authz: authz,
			})
			actualResult, err := svc.ExportRegulatoryReport(context.Background(), port.RegulatoryExportRequest{
				TenantID: tfrTenant, ReportType: tc.reportType, PeriodID: "2026-09",
				Fields: tc.fields, Actor: "u-1", IdempotencyKey: "key-export-1",
			})
			require.NoError(t, err)
			assert.Equal(t, command.ReportReady, actualResult.Status)
			assert.Contains(t, outboxTypes(uow), command.EventReportGenerated)
		})
	}
}
