package command_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func newIntentService(t *testing.T, uow port.UnitOfWork, store command.IntentStore, processor port.PaymentProcessor, authz port.Authorizer) *command.IntentService {
	svc, err := command.NewIntentService(command.IntentServiceParams{
		UoW:            uow,
		Intents:        store,
		Processor:      processor,
		Clock:          newMockClock(t, tfrAt),
		IDs:            newMockIDGenerator(t, tfrTestIDs(40)...),
		Authz:          authz,
		AuthExpiryDays: 7,
	})
	require.NoError(t, err)
	return svc
}

func intentTestCommand() port.PaymentIntentRequest {
	return port.PaymentIntentRequest{
		TenantID:       tfrTenant,
		LedgerID:       tfrLedger,
		AmountMinor:    5000,
		AssetCode:      tfrAsset,
		Method:         valueobject.MethodCard,
		IdempotencyKey: "key-intent-1",
		Actor:          "u-1",
	}
}

func TestIntentCreate(t *testing.T) {
	t.Parallel()

	baseReq := intentTestCommand()

	type testCase struct {
		name          string
		req           port.PaymentIntentRequest
		preload       func(uow port.UnitOfWork)
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid intent creates pending intent",
			req:           baseReq,
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  baseReq,
			preload: func(uow port.UnitOfWork) {
				fp := command.Fingerprint(
					baseReq.IdempotencyKey,
					string(baseReq.TenantID),
					string(baseReq.LedgerID),
					fmt.Sprintf("%d", baseReq.AmountMinor),
					string(baseReq.AssetCode),
					string(baseReq.Method),
				)
				setUOWIdem(uow, baseReq.IdempotencyKey, fp, []byte("{corrupt-json"))
			},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger returns LEDGER_REQUIRED",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.LedgerID = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "zero amount returns INVALID_INTENT_AMOUNT",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.AmountMinor = 0
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("INVALID_INTENT_AMOUNT", "intent amount must be positive"),
		},
		{
			name: "negative amount returns INVALID_INTENT_AMOUNT",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.AmountMinor = -500
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("INVALID_INTENT_AMOUNT", "intent amount must be positive"),
		},
		{
			name: "missing asset code returns INTENT_ASSET_REQUIRED",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.AssetCode = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("INTENT_ASSET_REQUIRED", "intent requires an asset code"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.Actor = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "payment actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.PaymentIntentRequest {
				r := baseReq
				r.IdempotencyKey = ""
				return r
			}(),
			preload:       func(_ port.UnitOfWork) {},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payment requires an idempotency key"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			req:           baseReq,
			preload:       func(_ port.UnitOfWork) {},
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			if tc.preload != nil {
				tc.preload(uow)
			}
			store := newMockIntentStore(t)
			processor := newMockPaymentProcessor(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|payment.create|ledger/"+string(tc.req.LedgerID))
			}
			svc := newIntentService(t, uow, store, processor, authz)

			res, err := svc.CreateIntent(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, command.IntentPending, res.Status)
				assert.NotEmpty(t, res.IntentID)

				// Idempotency replay
				replayRes, replayErr := svc.CreateIntent(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.IntentID, replayRes.IntentID)
			}
		})
	}
}

func TestIntentConfirm(t *testing.T) {
	t.Parallel()

	baseConfirm := port.ConfirmIntentRequest{
		TenantID:       tfrTenant,
		IntentID:       "intent-seed-1",
		CaptureMinor:   5000,
		IdempotencyKey: "key-confirm-1",
		Actor:          "u-1",
	}

	type testCase struct {
		name           string
		req            port.ConfirmIntentRequest
		seedStatus     string
		processorRes   port.ChargeResult
		processorErr   error
		mutateOnCharge bool
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "successful capture confirms intent",
			req:            baseConfirm,
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED", TraceID: "trace-1"},
			processorErr:   nil,
			denied:         false,
			expectedStatus: command.IntentConfirmed,
			expectedError:  nil,
		},
		{
			name:           "processor requires action moves intent to REQUIRES_ACTION",
			req:            baseConfirm,
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "PENDING", RequiresAction: true},
			processorErr:   nil,
			denied:         false,
			expectedStatus: command.IntentRequiresAction,
			expectedError:  nil,
		},
		{
			name:           "processor down returns error and leaves pending",
			req:            baseConfirm,
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   errors.New("processor down"),
			denied:         false,
			expectedStatus: "",
			expectedError:  errors.New("processor down"),
		},
		{
			name: "over-capture exceeds authorized amount",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.CaptureMinor = 6000
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED"},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  &entity.Error{Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount; remaining=5000"},
		},
		{
			name: "missing intent ID returns INTENT_ID_REQUIRED",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.IntentID = ""
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_ID_REQUIRED", "intent id is required"),
		},
		{
			name: "zero capture amount returns INVALID_CAPTURE_AMOUNT",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.CaptureMinor = 0
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INVALID_CAPTURE_AMOUNT", "capture amount must be positive"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.Actor = ""
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "payment actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.IdempotencyKey = ""
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payment requires an idempotency key"),
		},
		{
			name: "intent not found returns INTENT_NOT_FOUND",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.IntentID = "non-existent"
				return r
			}(),
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
		{
			name: "already confirmed intent rejects new key",
			req: func() port.ConfirmIntentRequest {
				r := baseConfirm
				r.IdempotencyKey = "key-confirm-fresh"
				return r
			}(),
			seedStatus:     command.IntentConfirmed,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_STATE_INVALID", "only pending intents can be confirmed"),
		},
		{
			name:           "concurrent intent modification detected during transition",
			req:            baseConfirm,
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED"},
			processorErr:   nil,
			mutateOnCharge: true,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_CONCURRENT_MODIFICATION", "intent changed during processing; resubmit with the same key"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			req:            baseConfirm,
			seedStatus:     command.IntentPending,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockIntentStore(t)
			var onCharge func()
			if tc.mutateOnCharge {
				onCharge = func() {
					_ = store.UpdateIntent(context.Background(), command.IntentRecord{
						ID:       tc.req.IntentID,
						TenantID: tc.req.TenantID,
						Status:   command.IntentCanceled,
					})
				}
			}
			processor := newMockPaymentProcessor(t, mockProcessorParams{
				ChargeResult: tc.processorRes,
				ChargeErr:    tc.processorErr,
				OnCharge:     onCharge,
			})
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.req.Actor+"|payment.confirm|intent/"+tc.req.IntentID)
			}

			// Preload intent
			_ = store.CreateIntent(context.Background(), command.IntentRecord{
				ID:          baseConfirm.IntentID,
				TenantID:    tfrTenant,
				LedgerID:    tfrLedger,
				AmountMinor: 5000,
				AssetCode:   tfrAsset,
				Method:      valueobject.MethodCard,
				Status:      tc.seedStatus,
				AuthID:      "auth-1",
				ProviderID:  "pr-1",
			})

			svc := newIntentService(t, uow, store, processor, authz)

			res, err := svc.ConfirmIntent(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, res.Status)

				// Idempotency replay
				replayRes, replayErr := svc.ConfirmIntent(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.IntentID, replayRes.IntentID)
			}
		})
	}
}

func TestIntentCancel(t *testing.T) {
	t.Parallel()

	baseQuery := port.PaymentQuery{
		TenantID: string(tfrTenant),
		ID:       "intent-cancel-1",
		Actor:    "u-1",
	}

	type testCase struct {
		name          string
		query         port.PaymentQuery
		seedStatus    string
		seedIntent    bool
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "canceling pending intent succeeds",
			query:         baseQuery,
			seedStatus:    command.IntentPending,
			seedIntent:    true,
			denied:        false,
			expectedError: nil,
		},
		{
			name:          "canceling requires_action intent succeeds",
			query:         baseQuery,
			seedStatus:    command.IntentRequiresAction,
			seedIntent:    true,
			denied:        false,
			expectedError: nil,
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			query: func() port.PaymentQuery {
				q := baseQuery
				q.Actor = ""
				return q
			}(),
			seedStatus:    command.IntentPending,
			seedIntent:    true,
			denied:        false,
			expectedError: entity.NewError("ACTOR_REQUIRED", "payment actor is required"),
		},
		{
			name:          "intent not found returns INTENT_NOT_FOUND",
			query:         baseQuery,
			seedStatus:    command.IntentPending,
			seedIntent:    false,
			denied:        false,
			expectedError: entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
		{
			name:          "already confirmed intent cannot be canceled",
			query:         baseQuery,
			seedStatus:    command.IntentConfirmed,
			seedIntent:    true,
			denied:        false,
			expectedError: entity.NewError("INTENT_STATE_INVALID", "only uncaptured intents can be canceled"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			query:         baseQuery,
			seedStatus:    command.IntentPending,
			seedIntent:    true,
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockIntentStore(t)
			processor := newMockPaymentProcessor(t)
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, tc.query.Actor+"|payment.cancel|intent/"+tc.query.ID)
			}

			if tc.seedIntent {
				_ = store.CreateIntent(context.Background(), command.IntentRecord{
					ID:          tc.query.ID,
					TenantID:    valueobject.TenantID(tc.query.TenantID),
					LedgerID:    tfrLedger,
					AmountMinor: 5000,
					AssetCode:   tfrAsset,
					Method:      valueobject.MethodCard,
					Status:      tc.seedStatus,
				})
			}

			svc := newIntentService(t, uow, store, processor, authz)

			res, err := svc.CancelIntent(context.Background(), tc.query)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, command.IntentCanceled, res.Status)

				// Idempotency replay
				replayRes, replayErr := svc.CancelIntent(context.Background(), tc.query)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.IntentID, replayRes.IntentID)
			}
		})
	}
}

func TestIntentCompleteChallenge(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		intentID       string
		succeeded      bool
		seedStatus     string
		seedIntent     bool
		processorRes   port.ChargeResult
		processorErr   error
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "challenge success confirms intent",
			intentID:       "intent-chal-1",
			succeeded:      true,
			seedStatus:     command.IntentRequiresAction,
			seedIntent:     true,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED"},
			processorErr:   nil,
			denied:         false,
			expectedStatus: command.IntentConfirmed,
			expectedError:  nil,
		},
		{
			name:           "challenge failure marks intent failed",
			intentID:       "intent-chal-2",
			succeeded:      false,
			seedStatus:     command.IntentRequiresAction,
			seedIntent:     true,
			processorRes:   port.ChargeResult{ProviderID: "pr-1", Status: "FAILED"},
			processorErr:   nil,
			denied:         false,
			expectedStatus: command.IntentFailed,
			expectedError:  nil,
		},
		{
			name:           "intent not found returns INTENT_NOT_FOUND",
			intentID:       "non-existent",
			succeeded:      true,
			seedStatus:     command.IntentRequiresAction,
			seedIntent:     false,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
		{
			name:           "pending intent cannot complete challenge",
			intentID:       "intent-chal-3",
			succeeded:      true,
			seedStatus:     command.IntentPending,
			seedIntent:     true,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_STATE_INVALID", "only challenged intents can complete"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			intentID:       "intent-chal-4",
			succeeded:      true,
			seedStatus:     command.IntentRequiresAction,
			seedIntent:     true,
			processorRes:   port.ChargeResult{},
			processorErr:   nil,
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockIntentStore(t)
			processor := newMockPaymentProcessor(t, mockProcessorParams{
				ChallengeResult: tc.processorRes,
				ChallengeErr:    tc.processorErr,
			})
			authz := newMockAuthorizer(t)
			if tc.denied {
				setAuthzDenied(authz, "u-1|payment.confirm|intent/"+tc.intentID)
			}

			if tc.seedIntent {
				_ = store.CreateIntent(context.Background(), command.IntentRecord{
					ID:          tc.intentID,
					TenantID:    tfrTenant,
					LedgerID:    tfrLedger,
					AmountMinor: 5000,
					AssetCode:   tfrAsset,
					Method:      valueobject.MethodCard,
					Status:      tc.seedStatus,
					AuthID:      "auth-1",
					ProviderID:  "pr-1",
				})
			}

			svc := newIntentService(t, uow, store, processor, authz)

			res, err := svc.CompleteChallenge(context.Background(), tfrTenant, tc.intentID, tc.succeeded, "u-1", "key-chal-"+tc.intentID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, res.Status)

				// Idempotency replay
				replayRes, replayErr := svc.CompleteChallenge(context.Background(), tfrTenant, tc.intentID, tc.succeeded, "u-1", "key-chal-"+tc.intentID)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.IntentID, replayRes.IntentID)
			}
		})
	}
}

func TestIntentGetAndList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		queryID       string
		seedIntent    bool
		listLimit     int
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "get existing intent returns result",
			queryID:       "intent-get-1",
			seedIntent:    true,
			listLimit:     0,
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "get missing intent returns INTENT_NOT_FOUND",
			queryID:       "intent-missing",
			seedIntent:    false,
			listLimit:     0,
			expectedCount: 0,
			expectedError: entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
		{
			name:          "list intents respects limit and clamp",
			queryID:       "",
			seedIntent:    true,
			listLimit:     10,
			expectedCount: 1,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := newMockUOW(t)
			store := newMockIntentStore(t)
			processor := newMockPaymentProcessor(t)
			authz := newMockAuthorizer(t)

			if tc.seedIntent {
				_ = store.CreateIntent(context.Background(), command.IntentRecord{
					ID:          "intent-get-1",
					TenantID:    tfrTenant,
					LedgerID:    tfrLedger,
					AmountMinor: 5000,
					AssetCode:   tfrAsset,
					Method:      valueobject.MethodCard,
					Status:      command.IntentConfirmed,
				})
			}

			svc := newIntentService(t, uow, store, processor, authz)

			if tc.queryID != "" {
				res, err := svc.GetIntent(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: tc.queryID})
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, tc.queryID, res.IntentID)
				}
			} else {
				list, err := svc.ListIntents(context.Background(), tfrTenant, tc.listLimit)
				assert.NoError(t, err)
				assert.Len(t, list, tc.expectedCount)
			}
		})
	}
}

func TestNewIntentServiceValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		authExpiryDays int
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "positive expiry accepted",
			authExpiryDays: 7,
			expectedError:  nil,
		},
		{
			name:           "zero expiry rejected",
			authExpiryDays: 0,
			expectedError:  errors.New("payment: auth expiry days must be positive"),
		},
		{
			name:           "negative expiry rejected",
			authExpiryDays: -1,
			expectedError:  errors.New("payment: auth expiry days must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := command.NewIntentService(command.IntentServiceParams{
				AuthExpiryDays: tc.authExpiryDays,
			})
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, svc)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, svc)
			}
		})
	}
}
