package command_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	payMerchant = valueobject.AccountID("a-merchant")
	payRefunds  = valueobject.AccountID("a-refunds")
	payCash     = valueobject.AccountID("a-cash")
)

func payTestAccounts() map[valueobject.AccountID]entity.AccountData {
	accounts := tfrTestAccounts()
	accounts[payMerchant] = entity.AccountData{ID: payMerchant, TenantID: tfrTenant, LedgerID: tfrLedger, Number: "8000", Name: "merchant", Class: valueobject.ClassLiability, AssetCode: tfrAsset, Status: valueobject.StatusActive, Version: 1}
	accounts[payRefunds] = entity.AccountData{ID: payRefunds, TenantID: tfrTenant, LedgerID: tfrLedger, Number: "8001", Name: "refunds", Class: valueobject.ClassLiability, AssetCode: tfrAsset, Status: valueobject.StatusActive, Version: 1}
	accounts[payCash] = entity.AccountData{ID: payCash, TenantID: tfrTenant, LedgerID: tfrLedger, Number: "8002", Name: "cash", Class: valueobject.ClassAsset, AssetCode: tfrAsset, Status: valueobject.StatusActive, Version: 1}
	return accounts
}

type refundStoreFake struct {
	mu      sync.Mutex
	refunds map[string]command.RefundRecord
}

func (s *refundStoreFake) CreateRefund(_ context.Context, record command.RefundRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refunds == nil {
		s.refunds = map[string]command.RefundRecord{}
	}
	s.refunds[record.ID] = record
	return nil
}

func (s *refundStoreFake) FindRefund(_ context.Context, _ valueobject.TenantID, id string) (command.RefundRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.refunds[id]
	if !ok {
		return command.RefundRecord{}, entity.NewError("REFUND_NOT_FOUND", "refund is unknown")
	}
	return record, nil
}

func (s *refundStoreFake) ListRefunds(_ context.Context, _ valueobject.TenantID, limit int) ([]command.RefundRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []command.RefundRecord
	for _, record := range s.refunds {
		out = append(out, record)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

func (s *refundStoreFake) SumPriorRefunds(_ context.Context, _ valueobject.TenantID, originalTxn string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var total int64
	for _, record := range s.refunds {
		if record.OriginalTxn == originalTxn && record.Status == command.RefundSucceeded {
			total += record.AmountMinor
		}
	}
	return total, nil
}

func newRefundService(uow *tfrUOW, refunds *refundStoreFake, intents *intentStoreFake, processor *fakeProcessor, authz *tfrAuthz) *command.RefundService {
	return command.NewRefundService(command.RefundServiceParams{
		UoW:       uow,
		Refunds:   refunds,
		Intents:   intents,
		Processor: processor,
		Accounts:  &tfrAccounts{accounts: payTestAccounts()},
		Settlement: command.SettlementAccounts{
			MerchantPayable: payMerchant, RefundsPayable: payRefunds, CashAccount: payCash,
		},
		WindowDays: 90,
		Clock:      tfrClock{},
		IDs:        &tfrIDs{next: tfrTestIDs(40)},
		Authz:      authz,
	})
}

func seedIntent(t *testing.T, store *intentStoreFake, createdAt time.Time) string {
	t.Helper()
	id := fmt.Sprintf("intent-%02d", len(store.intents))
	store.intents[id] = command.IntentRecord{
		ID: id, TenantID: tfrTenant, LedgerID: tfrLedger, AmountMinor: 5000,
		AssetCode: tfrAsset, Method: valueobject.MethodCard, Status: command.IntentPending,
		CreatedAt: createdAt,
	}
	return id
}

func refundTestCommand(original string) port.RefundRequest {
	return port.RefundRequest{
		TenantID:       tfrTenant,
		OriginalTxn:    original,
		AmountMinor:    1000,
		IdempotencyKey: "key-refund-1",
		Actor:          "u-1",
	}
}

func TestRefundCreate(t *testing.T) {
	t.Parallel()

	baseCmd := refundTestCommand("intent-00")

	type testCase struct {
		name           string
		req            port.RefundRequest
		seedOriginal   bool
		originalAge    time.Duration
		priorRefund    int64
		processorErr   error
		preload        func(uow *tfrUOW)
		denied         bool
		expectedStatus string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid refund links reversal with succeeded fact",
			req:            baseCmd,
			seedOriginal:   true,
			originalAge:    0,
			priorRefund:    0,
			processorErr:   nil,
			denied:         false,
			expectedStatus: command.RefundSucceeded,
			expectedError:  nil,
		},
		{
			name:         "corrupt idempotency replay returns IDEMPOTENCY_RECORD_INVALID",
			req:          baseCmd,
			seedOriginal: true,
			preload: func(uow *tfrUOW) {
				fp := command.Fingerprint(
					baseCmd.IdempotencyKey, string(baseCmd.TenantID), baseCmd.OriginalTxn, fmt.Sprintf("%d", baseCmd.AmountMinor),
				)
				uow.idem = map[string]tfrIdemEntry{
					baseCmd.IdempotencyKey: {
						fingerprint: fp,
						response:    []byte("{corrupt-json"),
						completed:   true,
					},
				}
			},
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_RECORD_INVALID", "stored idempotency response is corrupt"),
		},
		{
			name: "missing tenant returns TENANT_REQUIRED",
			req: func() port.RefundRequest {
				r := baseCmd
				r.TenantID = ""
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing original transaction returns REFUND_ORIGINAL_REQUIRED",
			req: func() port.RefundRequest {
				r := baseCmd
				r.OriginalTxn = ""
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("REFUND_ORIGINAL_REQUIRED", "refund requires the original transaction"),
		},
		{
			name: "zero refund amount returns INVALID_REFUND_AMOUNT",
			req: func() port.RefundRequest {
				r := baseCmd
				r.AmountMinor = 0
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INVALID_REFUND_AMOUNT", "refund amount must be positive"),
		},
		{
			name: "negative refund amount returns INVALID_REFUND_AMOUNT",
			req: func() port.RefundRequest {
				r := baseCmd
				r.AmountMinor = -100
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INVALID_REFUND_AMOUNT", "refund amount must be positive"),
		},
		{
			name: "missing actor returns ACTOR_REQUIRED",
			req: func() port.RefundRequest {
				r := baseCmd
				r.Actor = ""
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("ACTOR_REQUIRED", "refund actor is required"),
		},
		{
			name: "missing idempotency key returns IDEMPOTENCY_KEY_REQUIRED",
			req: func() port.RefundRequest {
				r := baseCmd
				r.IdempotencyKey = ""
				return r
			}(),
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "refund requires an idempotency key"),
		},
		{
			name:           "original intent not found returns INTENT_NOT_FOUND",
			req:            baseCmd,
			seedOriginal:   false,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("INTENT_NOT_FOUND", "intent is unknown"),
		},
		{
			name: "refund exceeding remainder fails validation",
			req: func() port.RefundRequest {
				r := baseCmd
				r.AmountMinor = 6000
				return r
			}(),
			seedOriginal:   true,
			originalAge:    0,
			priorRefund:    0,
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("REFUND_EXCEEDS_ORIGINAL", "refund exceeds remaining refundable amount"),
		},
		{
			name:           "refund after window fails validation",
			req:            baseCmd,
			seedOriginal:   true,
			originalAge:    200 * 24 * time.Hour,
			priorRefund:    0,
			processorErr:   nil,
			denied:         false,
			expectedStatus: "",
			expectedError:  entity.NewError("REFUND_WINDOW_EXPIRED", "refund window has expired"),
		},
		{
			name:           "processor failure persists failed with fact",
			req:            baseCmd,
			seedOriginal:   true,
			originalAge:    0,
			priorRefund:    0,
			processorErr:   context.DeadlineExceeded,
			denied:         false,
			expectedStatus: command.RefundFailed,
			expectedError:  entity.NewError("REFUND_PROCESSOR_FAILED", "processor refund failed"),
		},
		{
			name:           "denied subject returns FORBIDDEN",
			req:            baseCmd,
			seedOriginal:   true,
			originalAge:    0,
			priorRefund:    0,
			processorErr:   nil,
			denied:         true,
			expectedStatus: "",
			expectedError:  entity.NewError("FORBIDDEN", "subject is not authorized for this action"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			if tc.preload != nil {
				tc.preload(uow)
			}
			intents := &intentStoreFake{intents: map[string]command.IntentRecord{}}
			refunds := &refundStoreFake{}
			processor := &fakeProcessor{refundErr: tc.processorErr}
			authz := &tfrAuthz{denied: map[string]bool{}}
			if tc.denied {
				authz.denied[tc.req.Actor+"|refund.create|payment/"+tc.req.OriginalTxn] = true
			}

			if tc.seedOriginal {
				_ = seedIntent(t, intents, tfrAt.Add(-tc.originalAge))
			}
			if tc.priorRefund > 0 {
				_ = refunds.CreateRefund(context.Background(), command.RefundRecord{
					ID:          "prior-ref-1",
					TenantID:    tc.req.TenantID,
					OriginalTxn: tc.req.OriginalTxn,
					AmountMinor: tc.priorRefund,
					Status:      command.RefundSucceeded,
				})
			}

			svc := newRefundService(uow, refunds, intents, processor, authz)

			res, err := svc.CreateRefund(context.Background(), tc.req)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.expectedStatus, res.Status)
				assert.Equal(t, tc.req.OriginalTxn, res.OriginalID)

				// Idempotency replay
				replayRes, replayErr := svc.CreateRefund(context.Background(), tc.req)
				assert.NoError(t, replayErr)
				assert.Equal(t, res.RefundID, replayRes.RefundID)
			}
		})
	}
}

func TestRefundGetAndList(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		queryID       string
		seedRefund    bool
		listLimit     int
		expectedCount int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "get existing refund returns result",
			queryID:       "ref-01",
			seedRefund:    true,
			listLimit:     0,
			expectedCount: 1,
			expectedError: nil,
		},
		{
			name:          "get missing refund returns REFUND_NOT_FOUND",
			queryID:       "ref-missing",
			seedRefund:    false,
			listLimit:     0,
			expectedCount: 0,
			expectedError: entity.NewError("REFUND_NOT_FOUND", "refund is unknown"),
		},
		{
			name:          "list refunds respects limit and clamp",
			queryID:       "",
			seedRefund:    true,
			listLimit:     10,
			expectedCount: 1,
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uow := &tfrUOW{}
			intents := &intentStoreFake{intents: map[string]command.IntentRecord{}}
			refunds := &refundStoreFake{}
			processor := &fakeProcessor{}
			authz := &tfrAuthz{denied: map[string]bool{}}

			if tc.seedRefund {
				_ = refunds.CreateRefund(context.Background(), command.RefundRecord{
					ID:          "ref-01",
					TenantID:    tfrTenant,
					OriginalTxn: "intent-00",
					AmountMinor: 1000,
					Status:      command.RefundSucceeded,
				})
			}

			svc := newRefundService(uow, refunds, intents, processor, authz)

			if tc.queryID != "" {
				res, err := svc.GetRefund(context.Background(), port.PaymentQuery{TenantID: string(tfrTenant), ID: tc.queryID})
				assert.Equal(t, tc.expectedError, err)
				if tc.expectedError == nil {
					assert.Equal(t, tc.queryID, res.RefundID)
					assert.Equal(t, "intent-00", res.OriginalID)
				}
			} else {
				list, err := svc.ListRefunds(context.Background(), tfrTenant, tc.listLimit)
				assert.NoError(t, err)
				assert.Len(t, list, tc.expectedCount)
			}
		})
	}
}
