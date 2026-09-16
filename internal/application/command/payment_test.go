package command_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

type fakeProcessor struct {
	mu              sync.Mutex
	chargeResult    port.ChargeResult
	chargeErr       error
	chargeKeys      []string
	onCharge        func()
	refundErr       error
	challengeResult port.ChargeResult
	challengeErr    error
	challengeCalls  int
}

func (f *fakeProcessor) Charge(_ context.Context, req port.ChargeRequest) (port.ChargeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.chargeKeys = append(f.chargeKeys, req.IdempotencyKey)
	if f.onCharge != nil {
		f.onCharge()
	}
	if f.chargeErr != nil {
		return port.ChargeResult{}, f.chargeErr
	}
	return f.chargeResult, nil
}

func (f *fakeProcessor) RefundCharge(_ context.Context, _ port.RefundChargeRequest) (port.ChargeResult, error) {
	if f.refundErr != nil {
		return port.ChargeResult{}, f.refundErr
	}
	return port.ChargeResult{ProviderID: "pr-1", Status: "SUCCEEDED", TraceID: "trace-1"}, nil
}

func (f *fakeProcessor) GetStatus(_ context.Context, _ valueobject.TenantID, _ string, _ time.Duration) (port.ChargeResult, error) {
	return f.chargeResult, f.chargeErr
}

func (f *fakeProcessor) CompleteChallenge(_ context.Context, _ port.ChallengeCompletion) (port.ChargeResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.challengeCalls++
	if f.challengeErr != nil {
		return port.ChargeResult{}, f.challengeErr
	}
	return f.challengeResult, nil
}

type intentStoreFake struct {
	mu      sync.Mutex
	intents map[string]command.IntentRecord
}

func (s *intentStoreFake) CreateIntent(_ context.Context, record command.IntentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.intents == nil {
		s.intents = map[string]command.IntentRecord{}
	}
	s.intents[record.ID] = record
	return nil
}

func (s *intentStoreFake) FindIntent(_ context.Context, _ valueobject.TenantID, id string) (command.IntentRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.intents[id]
	if !ok {
		return command.IntentRecord{}, entity.NewError("INTENT_NOT_FOUND", "intent is unknown")
	}
	return record, nil
}

func (s *intentStoreFake) ListIntents(_ context.Context, _ valueobject.TenantID, limit int) ([]command.IntentRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.IntentRecord
	for _, record := range s.intents {
		out = append(out, record)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *intentStoreFake) UpdateIntent(_ context.Context, record command.IntentRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.intents[record.ID] = record
	return nil
}

func newIntentService(uow *tfrUOW, store *intentStoreFake, processor *fakeProcessor, authz *tfrAuthz) *command.IntentService {
	return command.NewIntentService(command.IntentServiceParams{
		UoW:       uow,
		Intents:   store,
		Processor: processor,
		Clock:     tfrClock{},
		IDs:       &tfrIDs{next: tfrTestIDs(40)},
		Authz:     authz,
	})
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
		preload       func(uow *tfrUOW)
		denied        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid intent creates pending intent",
			req:           baseReq,
			preload:       func(_ *tfrUOW) {},
			denied:        false,
			expectedError: nil,
		},
		{
			name: "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:  baseReq,
			preload: func(uow *tfrUOW) {
				fp := command.Fingerprint(
					baseReq.IdempotencyKey,
					string(baseReq.TenantID),
					string(baseReq.LedgerID),
					fmt.Sprintf("%d", baseReq.AmountMinor),
					string(baseReq.AssetCode),
					string(baseReq.Method),
				)
				uow.idem = map[string]tfrIdemEntry{
					baseReq.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
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
			preload:       func(_ *tfrUOW) {},
			denied:        false,
			expectedError: entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payment requires an idempotency key"),
		},
		{
			name:          "denied subject returns FORBIDDEN",
			req:           baseReq,
			preload:       func(_ *tfrUOW) {},
			denied:        true,
			expectedError: entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			if tc.preload != nil {
				tc.preload(uow)
			}
			store := &intentStoreFake{}
			processor := &fakeProcessor{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.req.Actor+"|payment.create|ledger/"+string(tc.req.LedgerID)] = true
			}
			svc := newIntentService(uow, store, processor, authz)

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
			uow := &tfrUOW{}
			store := &intentStoreFake{}
			processor := &fakeProcessor{chargeResult: tc.processorRes, chargeErr: tc.processorErr}
			if tc.mutateOnCharge {
				processor.onCharge = func() {
					_ = store.UpdateIntent(context.Background(), command.IntentRecord{
						ID:       tc.req.IntentID,
						TenantID: tc.req.TenantID,
						Status:   command.IntentCanceled,
					})
				}
			}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.req.Actor+"|payment.confirm|intent/"+tc.req.IntentID] = true
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

			svc := newIntentService(uow, store, processor, authz)

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
			uow := &tfrUOW{}
			store := &intentStoreFake{}
			processor := &fakeProcessor{}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.query.Actor+"|payment.cancel|intent/"+tc.query.ID] = true
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

			svc := newIntentService(uow, store, processor, authz)

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
			uow := &tfrUOW{}
			store := &intentStoreFake{}
			processor := &fakeProcessor{challengeResult: tc.processorRes, challengeErr: tc.processorErr}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied["u-1|payment.confirm|intent/"+tc.intentID] = true
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

			svc := newIntentService(uow, store, processor, authz)

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
			uow := &tfrUOW{}
			store := &intentStoreFake{}
			processor := &fakeProcessor{}
			authz := &tfrAuthz{denied: map[string]bool{}}

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

			svc := newIntentService(uow, store, processor, authz)

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

func outboxTypes(uow *tfrUOW) []string {
	uow.mu.Lock()
	defer uow.mu.Unlock()
	types := make([]string, 0, len(uow.outbox))
	for _, fact := range uow.outbox {
		types = append(types, fact.EventType)
	}
	return types
}

func outboxPayload(t *testing.T, uow *tfrUOW, eventType string) map[string]string {
	t.Helper()
	uow.mu.Lock()
	defer uow.mu.Unlock()
	for _, fact := range uow.outbox {
		if fact.EventType == eventType {
			var payload map[string]string
			require.NoError(t, jsonparser.Unmarshal(fact.Payload, &payload))
			return payload
		}
	}
	t.Fatalf("no outbox fact %s", eventType)
	return nil
}
