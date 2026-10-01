package command_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
)

func eligiblePolicy() valueobject.PayoutPolicy {
	return valueobject.PayoutPolicy{
		TenantID: tfrTenant, AssetCode: tfrAsset, MinimumMinor: 1000,
		FirstPayoutHoldDays: 7, ReserveBPS: 0, InstantEligible: true, Version: "v1",
	}
}

func newPayoutService(t *testing.T, uow port.UnitOfWork, store command.PayoutStore, available int64, authz port.Authorizer) *command.PayoutService {
	return command.NewPayoutService(command.PayoutServiceParams{
		UoW:            uow,
		Payouts:        store,
		Balances:       newMockBalances(t, map[valueobject.AccountID]int64{tfrSrc: available}),
		TransitAccount: "a-transit",
		Clock:          newMockClock(t, tfrAt),
		IDs:            newMockIDGenerator(t, tfrTestIDs(40)...),
		Authz:          authz,
	})
}

func payoutTestCommand() port.PayoutRequest {
	return port.PayoutRequest{
		TenantID:            tfrTenant,
		LedgerID:            tfrLedger,
		AccountID:           tfrSrc,
		AmountMinor:         5000,
		AssetCode:           tfrAsset,
		Method:              valueobject.PayoutACH,
		DestinationVerified: true,
		TenantAgeDays:       30,
		IdempotencyKey:      "key-payout-1",
		Actor:               "u-1",
	}
}

func TestPayoutCreate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		mutate          func(req *port.PayoutRequest)
		available       int64
		preload         func(uow port.UnitOfWork, svc *command.PayoutService)
		expectedStatus  string
		expectedError   error
		expectedRecords int
	}

	testCases := []testCase{
		{
			name:            "eligible payout stages pending with created fact",
			mutate:          func(_ *port.PayoutRequest) {},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  command.PayoutPending,
			expectedError:   nil,
			expectedRecords: 1,
		},
		{
			name:      "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			mutate:    func(_ *port.PayoutRequest) {},
			available: 9000,
			preload: func(uow port.UnitOfWork, _ *command.PayoutService) {
				req := payoutTestCommand()
				fp := command.Fingerprint(
					req.IdempotencyKey, string(req.TenantID), string(req.AccountID),
					fmt.Sprintf("%d", req.AmountMinor), string(req.AssetCode), string(req.Method),
				)
				setUOWIdem(uow, req.IdempotencyKey, fp, []byte("{corrupt-json"))
			},
			expectedStatus:  "",
			expectedError:   entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedRecords: 0,
		},
		{
			name:      "duplicate replay returns original record",
			mutate:    func(_ *port.PayoutRequest) {},
			available: 9000,
			preload: func(_ port.UnitOfWork, svc *command.PayoutService) {
				_, err := svc.CreatePayout(context.Background(), payoutTestCommand())
				require.NoError(t, err)
			},
			expectedStatus:  command.PayoutPending,
			expectedError:   nil,
			expectedRecords: 1,
		},
		{
			name: "missing tenant fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.TenantID = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("TENANT_REQUIRED", "tenant id is required"),
			expectedRecords: 0,
		},
		{
			name: "missing ledger fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.LedgerID = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
			expectedRecords: 0,
		},
		{
			name: "missing account fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.AccountID = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("PAYOUT_ACCOUNT_REQUIRED", "payout requires an account id"),
			expectedRecords: 0,
		},
		{
			name: "zero amount fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.AmountMinor = 0
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("INVALID_PAYOUT_AMOUNT", "payout amount must be positive"),
			expectedRecords: 0,
		},
		{
			name: "missing asset fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.AssetCode = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("PAYOUT_ASSET_REQUIRED", "payout requires an asset code"),
			expectedRecords: 0,
		},
		{
			name: "missing actor fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.Actor = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("ACTOR_REQUIRED", "payout actor is required"),
			expectedRecords: 0,
		},
		{
			name: "missing idempotency key fails envelope validation",
			mutate: func(req *port.PayoutRequest) {
				req.IdempotencyKey = ""
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payout requires an idempotency key"),
			expectedRecords: 0,
		},
		{
			name: "below-minimum payout blocked without submission",
			mutate: func(req *port.PayoutRequest) {
				req.AmountMinor = 100
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("PAYOUT_BLOCKED", "payout blocked (BELOW_MINIMUM)"),
			expectedRecords: 0,
		},
		{
			name: "unverified destination blocked without submission",
			mutate: func(req *port.PayoutRequest) {
				req.DestinationVerified = false
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("PAYOUT_BLOCKED", "payout blocked (DESTINATION_UNVERIFIED)"),
			expectedRecords: 0,
		},
		{
			name: "first-payout hold blocked without submission",
			mutate: func(req *port.PayoutRequest) {
				req.TenantAgeDays = 1
			},
			available:       9000,
			preload:         func(_ port.UnitOfWork, _ *command.PayoutService) {},
			expectedStatus:  "",
			expectedError:   entity.NewError("PAYOUT_BLOCKED", "payout blocked (FIRST_PAYOUT_HOLD)"),
			expectedRecords: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockPayoutStore(t, eligiblePolicy())
			authz := newMockAuthorizer(t)
			svc := newPayoutService(t, uow, store, tc.available, authz)
			cmd := payoutTestCommand()
			tc.mutate(&cmd)
			tc.preload(uow, svc)
			actualResult, err := svc.CreatePayout(context.Background(), cmd)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
			store.AssertNumberOfCalls(t, "CreatePayout", tc.expectedRecords)
		})
	}
}

func TestPayoutCancel(t *testing.T) {
	t.Parallel()

	seedPending := func(t *testing.T, svc *command.PayoutService) string {
		t.Helper()
		res, err := svc.CreatePayout(context.Background(), payoutTestCommand())
		require.NoError(t, err)
		return res.PayoutID
	}

	type testCase struct {
		name           string
		query          func(id string) port.PaymentQuery
		preload        func(t *testing.T, svc *command.PayoutService, id string)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "pending payout cancels",
			query: func(id string) port.PaymentQuery {
				return port.PaymentQuery{TenantID: string(tfrTenant), ID: id, Actor: "u-1"}
			},
			preload:        func(_ *testing.T, _ *command.PayoutService, _ string) {},
			denied:         false,
			expectedStatus: command.PayoutCanceled,
			expectedError:  nil,
		},
		{
			name: "missing actor fails",
			query: func(id string) port.PaymentQuery {
				return port.PaymentQuery{TenantID: string(tfrTenant), ID: id, Actor: ""}
			},
			preload:        func(_ *testing.T, _ *command.PayoutService, _ string) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "payout actor is required"),
		},
		{
			name: "replay cancel returns stored cancellation",
			query: func(id string) port.PaymentQuery {
				return port.PaymentQuery{TenantID: string(tfrTenant), ID: id, Actor: "u-1"}
			},
			preload: func(t *testing.T, svc *command.PayoutService, id string) {
				_, err := svc.CancelPayout(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: id, Actor: "u-1"})
				require.NoError(t, err)
			},
			denied:         false,
			expectedStatus: command.PayoutCanceled,
			expectedError:  nil,
		},
		{
			name: "payout not found fails",
			query: func(_ string) port.PaymentQuery {
				return port.PaymentQuery{TenantID: string(tfrTenant), ID: "nonexistent", Actor: "u-1"}
			},
			preload:        func(_ *testing.T, _ *command.PayoutService, _ string) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("PAYOUT_NOT_FOUND", "payout is unknown"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			query: func(id string) port.PaymentQuery {
				return port.PaymentQuery{TenantID: string(tfrTenant), ID: id, Actor: "u-1"}
			},
			preload:        func(_ *testing.T, _ *command.PayoutService, _ string) {},
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockPayoutStore(t, eligiblePolicy())
			authz := newMockAuthorizer(t)
			svc := newPayoutService(t, uow, store, 9000, authz)
			id := seedPending(t, svc)
			query := tc.query(id)
			if tc.denied {
				setAuthzDenied(authz, "u-1|payout.cancel|payout/"+id)
			}
			tc.preload(t, svc, id)
			actualResult, err := svc.CancelPayout(context.Background(), query)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
		})
	}
}

func TestTopupCreate(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T) (port.UnitOfWork, *mockcommand.MockTopupStore, port.Authorizer, *command.TopupService) {
		uow := newMockUOW(t)
		store := newMockTopupStore(t)
		authz := newMockAuthorizer(t)
		svc := command.NewTopupService(command.TopupServiceParams{
			UoW: uow, Topups: store, Clock: newMockClock(t, tfrAt),
			IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
		})
		return uow, store, authz, svc
	}

	topupCommand := func() port.TopupRequest {
		return port.TopupRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger, CreditAccount: tfrDst,
			AmountMinor: 5000, AssetCode: tfrAsset, InstrumentID: "bank-1",
			InstrumentVerified: true, IdempotencyKey: "key-topup-1", Actor: "u-1",
		}
	}

	type testCase struct {
		name           string
		req            port.TopupRequest
		preload        func(t *testing.T, uow port.UnitOfWork, svc *command.TopupService)
		denied         bool
		expectedStatus string
		expectedError  error
		expectedStore  int
	}

	testCases := []testCase{
		{
			name:           "verified instrument persists pending",
			req:            topupCommand(),
			preload:        func(_ *testing.T, _ port.UnitOfWork, _ *command.TopupService) {},
			denied:         false,
			expectedStatus: command.TopupPending,
			expectedError:  nil,
			expectedStore:  1,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  topupCommand(),
			preload: func(t *testing.T, uow port.UnitOfWork, _ *command.TopupService) {
				req := topupCommand()
				fp := command.Fingerprint(req.IdempotencyKey, string(req.TenantID), fmt.Sprintf("%d", req.AmountMinor))
				setUOWIdem(uow, req.IdempotencyKey, fp, []byte("{corrupt-json"))
			},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
			expectedStore:  0,
		},
		{
			name: "duplicate replay returns original topup",
			req:  topupCommand(),
			preload: func(t *testing.T, _ port.UnitOfWork, svc *command.TopupService) {
				_, err := svc.CreateTopup(context.Background(), topupCommand())
				require.NoError(t, err)
			},
			denied:         false,
			expectedStatus: command.TopupPending,
			expectedError:  nil,
			expectedStore:  1,
		},
		{
			name: "unverified instrument rejected",
			req: func() port.TopupRequest {
				c := topupCommand()
				c.InstrumentVerified = false
				return c
			}(),
			preload:        func(_ *testing.T, _ port.UnitOfWork, _ *command.TopupService) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACCOUNT_UNVERIFIED", "top-up requires a verified external bank instrument"),
			expectedStore:  0,
		},
		{
			name: "denied subject returns FORBIDDEN",
			req: func() port.TopupRequest {
				c := topupCommand()
				c.IdempotencyKey = "key-topup-denied"
				return c
			}(),
			preload:        func(_ *testing.T, _ port.UnitOfWork, _ *command.TopupService) {},
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
			expectedStore:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow, store, authz, svc := newSvc(t)
			if tc.denied {
				setAuthzDenied(authz, "u-1|topup.create|ledger/"+string(tfrLedger))
			}
			tc.preload(t, uow, svc)
			actualResult, err := svc.CreateTopup(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
			store.AssertNumberOfCalls(t, "CreateTopup", tc.expectedStore)
		})
	}
}

func TestTopupCancel(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T) (*mockcommand.MockTopupStore, port.Authorizer, *command.TopupService) {
		uow := newMockUOW(t)
		store := newMockTopupStore(t)
		authz := newMockAuthorizer(t)
		svc := command.NewTopupService(command.TopupServiceParams{
			UoW: uow, Topups: store, Clock: newMockClock(t, tfrAt),
			IDs: newMockIDGenerator(t, tfrTestIDs(40)...), Authz: authz,
		})
		return store, authz, svc
	}

	topupCommand := func() port.TopupRequest {
		return port.TopupRequest{
			TenantID: tfrTenant, LedgerID: tfrLedger, CreditAccount: tfrDst,
			AmountMinor: 5000, AssetCode: tfrAsset, InstrumentID: "bank-1",
			InstrumentVerified: true, IdempotencyKey: "key-topup-1", Actor: "u-1",
		}
	}

	seedPending := func(t *testing.T, svc *command.TopupService) string {
		t.Helper()
		res, err := svc.CreateTopup(context.Background(), topupCommand())
		require.NoError(t, err)
		return res.TopupID
	}

	type testCase struct {
		name           string
		topupID        func(id string) string
		actor          string
		preload        func(t *testing.T, svc *command.TopupService, id string)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "pending topup is canceled",
			topupID: func(id string) string {
				return id
			},
			actor:          "u-1",
			preload:        func(_ *testing.T, _ *command.TopupService, _ string) {},
			denied:         false,
			expectedStatus: command.TopupCanceled,
			expectedError:  nil,
		},
		{
			name: "second cancellation fails",
			topupID: func(id string) string {
				return id
			},
			actor: "u-1",
			preload: func(t *testing.T, svc *command.TopupService, id string) {
				_, err := svc.CancelTopup(context.Background(), tfrTenant, id, "u-1")
				require.NoError(t, err)
			},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("TOPUP_STATE_INVALID", "only pending top-ups can be canceled"),
		},
		{
			name: "topup not found fails",
			topupID: func(_ string) string {
				return "nonexistent"
			},
			actor:          "u-1",
			preload:        func(_ *testing.T, _ *command.TopupService, _ string) {},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("TOPUP_NOT_FOUND", "top-up is unknown"),
		},
		{
			name: "denied subject returns FORBIDDEN",
			topupID: func(id string) string {
				return id
			},
			actor:          "u-1",
			preload:        func(_ *testing.T, _ *command.TopupService, _ string) {},
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, authz, svc := newSvc(t)
			id := seedPending(t, svc)
			targetID := tc.topupID(id)
			if tc.denied {
				setAuthzDenied(authz, "u-1|topup.cancel|ledger/"+targetID)
			}
			tc.preload(t, svc, id)
			actualResult, err := svc.CancelTopup(context.Background(), tfrTenant, targetID, tc.actor)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, actualResult.Status)
			}
		})
	}
}

func TestWebhookEventNames(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		eventType    string
		expectedKeys []string
	}

	uow := newMockUOW(t)
	intentStore := newMockIntentStore(t)
	refundStore := newMockRefundStore(t)
	payoutStore := newMockPayoutStore(t, eligiblePolicy())
	authz := newMockAuthorizer(t)
	processor := newMockPaymentProcessor(t, mockProcessorParams{ChargeResult: port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED"}})

	intents := newIntentService(t, uow, intentStore, processor, authz)
	refunds := newRefundService(t, uow, refundStore, intentStore, processor, authz)
	payouts := newPayoutService(t, uow, payoutStore, 90000, authz)

	created, err := intents.CreateIntent(context.Background(), intentTestCommand())
	require.NoError(t, err)
	_, err = intents.ConfirmIntent(context.Background(), port.ConfirmIntentRequest{
		TenantID: tfrTenant, IntentID: created.IntentID, CaptureMinor: 5000,
		IdempotencyKey: "key-confirm-1", Actor: "u-1",
	})
	require.NoError(t, err)

	original := seedIntent(t, intentStore, tfrAt)
	_, err = refunds.CreateRefund(context.Background(), refundTestCommand(original))
	require.NoError(t, err)

	_, err = payouts.CreatePayout(context.Background(), payoutTestCommand())
	require.NoError(t, err)

	testCases := []testCase{
		{
			name:         "intent created matches §10",
			eventType:    "payment_intent.created.v1",
			expectedKeys: []string{"id", "status"},
		},
		{
			name:         "intent succeeded matches §10",
			eventType:    "payment_intent.succeeded.v1",
			expectedKeys: []string{"id", "status"},
		},
		{
			name:         "refund succeeded matches §10",
			eventType:    "refund.succeeded.v1",
			expectedKeys: []string{"id", "original", "status"},
		},
		{
			name:         "payout created matches §10",
			eventType:    "payout.created.v1",
			expectedKeys: []string{"id", "status"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := outboxPayload(t, uow, tc.eventType)
			for _, key := range tc.expectedKeys {
				assert.Contains(t, payload, key)
			}
		})
	}
}

func TestPayoutGetAndList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		queryID       string
		seedPayout    bool
		listLimit     int
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "get existing payout returns result",
			queryID:       "po-01",
			seedPayout:    true,
			listLimit:     0,
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "get missing payout returns PAYOUT_NOT_FOUND",
			queryID:       "po-missing",
			seedPayout:    false,
			listLimit:     0,
			expectedCount: 0,
			expectedError: entity.NewError("PAYOUT_NOT_FOUND", "payout is unknown"),
		},
		{
			name:          "list payouts respects limit",
			queryID:       "",
			seedPayout:    true,
			listLimit:     10,
			expectedCount: 1,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockPayoutStore(t, eligiblePolicy())
			authz := newMockAuthorizer(t)

			if tc.seedPayout {
				_ = store.CreatePayout(context.Background(), command.PayoutRecord{
					ID:          "po-01",
					TenantID:    tfrTenant,
					LedgerID:    tfrLedger,
					AccountID:   tfrSrc,
					AmountMinor: 5000,
					AssetCode:   tfrAsset,
					Method:      valueobject.PayoutACH,
					Status:      command.PayoutPending,
				})
			}

			svc := newPayoutService(t, uow, store, 90000, authz)

			if tc.queryID != "" {
				res, err := svc.GetPayout(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: tc.queryID})
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, tc.queryID, res.PayoutID)
				}
			} else {
				list, err := svc.ListPayouts(context.Background(), tfrTenant, tc.listLimit)
				assert.NoError(t, err)
				assert.Len(t, list, tc.expectedCount)
			}
		})
	}
}

func TestPayoutSchedule(t *testing.T) {
	t.Parallel()

	basePolicy := eligiblePolicy()

	type testCase struct {
		name          string
		policy        valueobject.PayoutPolicy
		actor         string
		key           string
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid schedule update succeeds",
			policy:        basePolicy,
			actor:         "u-1",
			key:           "key-sched-1",
			denied:        false,
			expectedError: nil,
		},
		{
			name: "invalid policy rejected",
			policy: func() valueobject.PayoutPolicy {
				p := basePolicy
				p.MinimumMinor = -100
				return p
			}(),
			actor:         "u-1",
			key:           "key-sched-2",
			denied:        false,
			expectedError: entity.NewError("PAYOUT_POLICY_INVALID", "payout policy is invalid"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			policy:        basePolicy,
			actor:         "u-1",
			key:           "key-sched-3",
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockPayoutStore(t, basePolicy)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.actor+"|payout.schedule|tenant/"+string(tfrTenant))
			}
			svc := newPayoutService(t, uow, store, 90000, authz)

			res, err := svc.UpdateSchedule(context.Background(), tfrTenant, tc.policy, tc.actor, tc.key)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.policy.MinimumMinor, res.MinimumMinor)

				// Replay
				replayRes, replayErr := svc.UpdateSchedule(context.Background(), tfrTenant, tc.policy, tc.actor, tc.key)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.Version, replayRes.Version)

				// GetSchedule verification
				readPolicy, readErr := svc.GetSchedule(context.Background(), tfrTenant, tc.policy.AssetCode)
				assert.NoError(t, readErr)
				assert.Equal(t, tc.policy.MinimumMinor, readPolicy.MinimumMinor)
			}
		})
	}
}

func TestTopupGetAndList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		queryID       string
		seedTopup     bool
		listLimit     int
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "get existing topup returns result",
			queryID:       "topup-01",
			seedTopup:     true,
			listLimit:     0,
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "get missing topup returns TOPUP_NOT_FOUND",
			queryID:       "topup-missing",
			seedTopup:     false,
			listLimit:     0,
			expectedCount: 0,
			expectedError: entity.NewError("TOPUP_NOT_FOUND", "top-up is unknown"),
		},
		{
			name:          "list topups respects limit and clamp",
			queryID:       "",
			seedTopup:     true,
			listLimit:     10,
			expectedCount: 1,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockTopupStore(t)
			authz := newMockAuthorizer(t)

			if tc.seedTopup {
				_ = store.CreateTopup(context.Background(), command.TopupRecord{
					ID:          "topup-01",
					TenantID:    tfrTenant,
					LedgerID:    tfrLedger,
					AccountID:   tfrDst,
					AmountMinor: 5000,
					AssetCode:   tfrAsset,
					Status:      command.TopupPending,
				})
			}

			svc := command.NewTopupService(command.TopupServiceParams{
				UoW:    uow,
				Topups: store,
				Clock:  newMockClock(t, tfrAt),
				IDs:    newMockIDGenerator(t, tfrTestIDs(40)...),
				Authz:  authz,
			})

			if tc.queryID != "" {
				res, err := svc.GetTopup(context.Background(), tfrTenant, tc.queryID)
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, tc.queryID, res.TopupID)
				}
			} else {
				list, err := svc.ListTopups(context.Background(), tfrTenant, tc.listLimit)
				assert.NoError(t, err)
				assert.Len(t, list, tc.expectedCount)
			}
		})
	}
}

func TestPaymentUseCasesComposite(t *testing.T) {
	t.Parallel()

	uow := newMockUOW(t)
	intentStore := newMockIntentStore(t)
	refundStore := newMockRefundStore(t)
	payoutStore := newMockPayoutStore(t, eligiblePolicy())
	topupStore := newMockTopupStore(t)
	authz := newMockAuthorizer(t)
	processor := newMockPaymentProcessor(t, mockProcessorParams{ChargeResult: port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED"}})

	intents := newIntentService(t, uow, intentStore, processor, authz)
	refunds := newRefundService(t, uow, refundStore, intentStore, processor, authz)
	payouts := newPayoutService(t, uow, payoutStore, 90000, authz)
	topups := command.NewTopupService(command.TopupServiceParams{
		UoW:    uow,
		Topups: topupStore,
		Clock:  newMockClock(t, tfrAt),
		IDs:    newMockIDGenerator(t, tfrTestIDs(40)...),
		Authz:  authz,
	})

	adapter := command.NewPaymentUseCases(command.PaymentUseCasesParams{
		Intents: intents,
		Refunds: refunds,
		Payouts: payouts,
		Topups:  topups,
	})

	// 1. Intent flow via composite
	intentRes, err := adapter.CreateIntent(context.Background(), intentTestCommand())
	require.NoError(t, err)
	assert.Equal(t, command.IntentPending, intentRes.Status)

	gotIntent, err := adapter.GetIntent(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: intentRes.IntentID})
	require.NoError(t, err)
	assert.Equal(t, intentRes.IntentID, gotIntent.IntentID)

	intentList, err := adapter.ListIntents(context.Background(), tfrTenant, 10)
	require.NoError(t, err)
	assert.Len(t, intentList, 1)

	confirmRes, err := adapter.ConfirmIntent(context.Background(), port.ConfirmIntentRequest{
		TenantID:       tfrTenant,
		IntentID:       intentRes.IntentID,
		CaptureMinor:   5000,
		IdempotencyKey: "key-comp-confirm",
		Actor:          "u-1",
	})
	require.NoError(t, err)
	assert.Equal(t, command.IntentConfirmed, confirmRes.Status)

	// 2. Refund flow via composite
	refundRes, err := adapter.CreateRefund(context.Background(), port.RefundRequest{
		TenantID:       tfrTenant,
		OriginalTxn:    intentRes.IntentID,
		AmountMinor:    1000,
		IdempotencyKey: "key-comp-refund",
		Actor:          "u-1",
	})
	require.NoError(t, err)
	assert.Equal(t, command.RefundSucceeded, refundRes.Status)

	gotRefund, err := adapter.GetRefund(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: refundRes.RefundID})
	require.NoError(t, err)
	assert.Equal(t, refundRes.RefundID, gotRefund.RefundID)

	refundList, err := adapter.ListRefunds(context.Background(), tfrTenant, 10)
	require.NoError(t, err)
	assert.Len(t, refundList, 1)

	// 3. Payout flow via composite
	payoutRes, err := adapter.CreatePayout(context.Background(), payoutTestCommand())
	require.NoError(t, err)
	assert.Equal(t, command.PayoutPending, payoutRes.Status)

	gotPayout, err := adapter.GetPayout(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: payoutRes.PayoutID})
	require.NoError(t, err)
	assert.Equal(t, payoutRes.PayoutID, gotPayout.PayoutID)

	payoutList, err := adapter.ListPayouts(context.Background(), tfrTenant, 10)
	require.NoError(t, err)
	assert.Len(t, payoutList, 1)

	cancelPayoutRes, err := adapter.CancelPayout(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: payoutRes.PayoutID, Actor: "u-1"})
	require.NoError(t, err)
	assert.Equal(t, command.PayoutCanceled, cancelPayoutRes.Status)

	// 4. Topup flow via composite
	topupRes, err := adapter.CreateTopup(context.Background(), port.TopupRequest{
		TenantID:           tfrTenant,
		LedgerID:           tfrLedger,
		CreditAccount:      tfrDst,
		AmountMinor:        5000,
		AssetCode:          tfrAsset,
		InstrumentID:       "bank-comp-1",
		InstrumentVerified: true,
		IdempotencyKey:     "key-comp-topup",
		Actor:              "u-1",
	})
	require.NoError(t, err)
	assert.Equal(t, command.TopupPending, topupRes.Status)

	gotTopup, err := adapter.GetTopup(context.Background(), tfrTenant, topupRes.TopupID)
	require.NoError(t, err)
	assert.Equal(t, topupRes.TopupID, gotTopup.TopupID)

	topupList, err := adapter.ListTopups(context.Background(), tfrTenant, 10)
	require.NoError(t, err)
	assert.Len(t, topupList, 1)

	// Cancel intent via composite
	newIntent, err := adapter.CreateIntent(context.Background(), port.PaymentIntentRequest{
		TenantID:       tfrTenant,
		LedgerID:       tfrLedger,
		AmountMinor:    3000,
		AssetCode:      tfrAsset,
		Method:         valueobject.MethodCard,
		IdempotencyKey: "key-comp-intent-2",
		Actor:          "u-1",
	})
	require.NoError(t, err)
	cancelIntentRes, err := adapter.CancelIntent(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: newIntent.IntentID, Actor: "u-1"})
	require.NoError(t, err)
	assert.Equal(t, command.IntentCanceled, cancelIntentRes.Status)
}
