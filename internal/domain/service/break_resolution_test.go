package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateResolution(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseReq := service.ResolutionRequest{
		BreakID:     "brk-1",
		Action:      valueobject.ResolveAcknowledge,
		ReasonCode:  "TIMING_SKEW",
		EvidenceRef: "ev-1",
		Owner:       "finance",
		Actor:       "maker-1",
		Approver:    "checker-1",
		AuditLink:   "audit-1",
		Now:         now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}

	type testCase struct {
		name           string
		req            service.ResolutionRequest
		current        valueobject.BreakStatus
		thresholdMinor int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid acknowledge from open",
			req:            baseReq,
			current:        valueobject.BreakOpen,
			thresholdMinor: 1000,
			expectedError:  nil,
		},
		{
			name: "self approval above threshold rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 5000
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "p-adj-1"
				r.Approver = r.Actor
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment approver must differ from maker"),
		},
		{
			name: "missing approver above threshold rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 5000
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "p-adj-1"
				r.Approver = ""
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment above threshold requires an approver"),
		},
		{
			name: "whitespace approver above threshold rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 5000
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "p-adj-1"
				r.Approver = "   "
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment above threshold requires an approver"),
		},
		{
			name: "at threshold self approval allowed",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 1000
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "p-adj-1"
				r.Approver = r.Actor
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  nil,
		},
		{
			name: "blank adjustment posting id rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 500
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "   "
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("EVIDENCE_REQUIRED", "adjustment requires the linked posting id"),
		},
		{
			name: "invalid action rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolutionAction("DELETE")
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("RESOLUTION_ACTION_INVALID", "resolution action is invalid"),
		},
		{
			name: "whitespace break id rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.BreakID = "   "
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("BREAK_ID_REQUIRED", "break id is required"),
		},
		{
			name: "negative threshold rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.Action = valueobject.ResolveAdjustLedger
				r.AmountMinor = 500
				r.AssetCode = "USD"
				r.AdjustmentPostingID = "p-adj-1"
				return r
			}(),
			current:        valueobject.BreakInReview,
			thresholdMinor: -1,
			expectedError:  entity.NewError("THRESHOLD_INVALID", "approval threshold must be non-negative"),
		},
		{
			name: "missing evidence rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.EvidenceRef = ""
				return r
			}(),
			current:        valueobject.BreakOpen,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("EVIDENCE_REQUIRED", "resolution requires reason, evidence, owner, actor, and audit link"),
		},
		{
			name: "past expiry rejected",
			req: func() service.ResolutionRequest {
				r := baseReq
				r.ExpiresAt = now.Add(-time.Hour)
				return r
			}(),
			current:        valueobject.BreakOpen,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("ACK_EXPIRY_INVALID", "acknowledgement expiry must be in the future"),
		},
		{
			name:           "illegal transition from acknowledged",
			req:            baseReq,
			current:        valueobject.BreakAcknowledged,
			thresholdMinor: 1000,
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "resolution action is not allowed from current status"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateResolution(tc.req, tc.current, tc.thresholdMinor)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestTransitionBreak(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		current        valueobject.BreakStatus
		action         valueobject.ResolutionAction
		expectedResult valueobject.BreakStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "open acknowledge enters review",
			current:        valueobject.BreakOpen,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: valueobject.BreakInReview,
			expectedError:  nil,
		},
		{
			name:           "review adjust resolves",
			current:        valueobject.BreakInReview,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: valueobject.BreakResolved,
			expectedError:  nil,
		},
		{
			name:           "review acknowledge acknowledges",
			current:        valueobject.BreakInReview,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: valueobject.BreakAcknowledged,
			expectedError:  nil,
		},
		{
			name:           "review escalate escalates",
			current:        valueobject.BreakInReview,
			action:         valueobject.ResolveEscalate,
			expectedResult: valueobject.BreakEscalated,
			expectedError:  nil,
		},
		{
			name:           "escalated adjust resolves",
			current:        valueobject.BreakEscalated,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: valueobject.BreakResolved,
			expectedError:  nil,
		},
		{
			name:           "open unknown action",
			current:        valueobject.BreakOpen,
			action:         valueobject.ResolutionAction("DELETE"),
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "unknown resolution action"),
		},
		{
			name:           "review unknown action",
			current:        valueobject.BreakInReview,
			action:         valueobject.ResolutionAction("DELETE"),
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "unknown resolution action"),
		},
		{
			name:           "escalated acknowledge forbidden",
			current:        valueobject.BreakEscalated,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "escalated breaks only resolve via adjustment or external error"),
		},
		{
			name:           "acknowledged frozen",
			current:        valueobject.BreakAcknowledged,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "break cannot transition from its current status"),
		},
		{
			name:           "resolved frozen",
			current:        valueobject.BreakResolved,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "break cannot transition from its current status"),
		},
		{
			name:           "unknown status frozen",
			current:        valueobject.BreakStatus("UNKNOWN"),
			action:         valueobject.ResolveAcknowledge,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "break cannot transition from its current status"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.TransitionBreak(tc.current, tc.action)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestIsAcknowledgementExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		expiresAt      time.Time
		now            time.Time
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "expired acknowledgement",
			expiresAt:      now.Add(-time.Hour),
			now:            now,
			expectedResult: true,
		},
		{
			name:           "active acknowledgement",
			expiresAt:      now.Add(time.Hour),
			now:            now,
			expectedResult: false,
		},
		{
			name:           "zero expiry never expires",
			expiresAt:      time.Time{},
			now:            now,
			expectedResult: false,
		},
		{
			name:           "exact boundary not expired",
			expiresAt:      now,
			now:            now,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := service.IsAcknowledgementExpired(tc.expiresAt, tc.now)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestReopenIfExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		current        valueobject.BreakStatus
		expiresAt      time.Time
		now            time.Time
		expectedResult valueobject.BreakStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "expired ack reopens",
			current:        valueobject.BreakAcknowledged,
			expiresAt:      now.Add(-time.Hour),
			now:            now,
			expectedResult: valueobject.BreakOpen,
			expectedError:  nil,
		},
		{
			name:           "active ack stays",
			current:        valueobject.BreakAcknowledged,
			expiresAt:      now.Add(time.Hour),
			now:            now,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ACK_ACTIVE", "acknowledgement has not expired"),
		},
		{
			name:           "non-ack never reopens",
			current:        valueobject.BreakInReview,
			expiresAt:      now.Add(-time.Hour),
			now:            now,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "only acknowledged breaks reopen on expiry"),
		},
		{
			name:           "resolved never reopens",
			current:        valueobject.BreakResolved,
			expiresAt:      now.Add(-time.Hour),
			now:            now,
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  entity.NewError("ILLEGAL_TRANSITION", "only acknowledged breaks reopen on expiry"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ReopenIfExpired(tc.current, tc.expiresAt, tc.now)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBuildAdjustmentLines(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		breakID           string
		originalPostingID string
		debitAccount      string
		creditAccount     string
		amountMinor       int64
		assetCode         string
		expectedResult    [2]service.AdjustmentLine
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "balanced adjustment",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "bank-cash",
			amountMinor:       2500,
			assetCode:         "USD",
			expectedResult: [2]service.AdjustmentLine{
				{AccountID: "suspense", Side: "DEBIT", AmountMinor: 2500, AssetCode: "USD"},
				{AccountID: "bank-cash", Side: "CREDIT", AmountMinor: 2500, AssetCode: "USD"},
			},
			expectedError: nil,
		},
		{
			name:              "same accounts rejected",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "suspense",
			amountMinor:       2500,
			assetCode:         "USD",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("ADJUSTMENT_UNBALANCED", "adjustment debit and credit must differ"),
		},
		{
			name:              "zero amount rejected",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "bank-cash",
			amountMinor:       0,
			assetCode:         "USD",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("INVALID_ENTRY_AMOUNT", "adjustment amount must be positive"),
		},
		{
			name:              "missing asset rejected",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "bank-cash",
			amountMinor:       100,
			assetCode:         "",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("ADJUSTMENT_ASSET_REQUIRED", "adjustment asset is required"),
		},
		{
			name:              "whitespace break id rejected",
			breakID:           "   ",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "bank-cash",
			amountMinor:       100,
			assetCode:         "USD",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("BREAK_ID_REQUIRED", "adjustment requires break and original posting ids"),
		},
		{
			name:              "whitespace accounts match rejected",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense ",
			creditAccount:     " suspense",
			amountMinor:       100,
			assetCode:         "USD",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("ADJUSTMENT_UNBALANCED", "adjustment debit and credit must differ"),
		},
		{
			name:              "negative amount rejected",
			breakID:           "brk-1",
			originalPostingID: "p-orig",
			debitAccount:      "suspense",
			creditAccount:     "bank-cash",
			amountMinor:       -500,
			assetCode:         "USD",
			expectedResult:    [2]service.AdjustmentLine{},
			expectedError:     entity.NewError("INVALID_ENTRY_AMOUNT", "adjustment amount must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildAdjustmentLines(tc.breakID, tc.originalPostingID, tc.debitAccount, tc.creditAccount, tc.amountMinor, tc.assetCode)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
