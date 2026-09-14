package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestOpenDispute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	policy := service.NetworkPolicy{
		Network:           "visa",
		Version:           "v2026.1",
		DisputeWindowDays: 120,
		EvidenceDays:      14,
		MaxRepresentments: 2,
		FeeMinor:          1500,
	}

	mkReq := func(mut func(*service.OpenDisputeRequest)) service.OpenDisputeRequest {
		req := service.OpenDisputeRequest{
			DisputeID:         "d-1",
			PaymentID:         "p-1",
			OriginalPostingID: "pst-1",
			Network:           "visa",
			AmountMinor:       5000,
			PaymentAt:         now.Add(-24 * time.Hour),
			Now:               now,
			HoldID:            "h-1",
			Policy:            policy,
		}
		mut(&req)
		return req
	}
	noop := func(*service.OpenDisputeRequest) {}

	okDispute := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name           string
		req            service.OpenDisputeRequest
		expectedResult entity.Dispute
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ok",
			req:            mkReq(noop),
			expectedResult: okDispute,
			expectedError:  nil,
		},
		{
			name: "past window",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.DisputeID = "d-late"
				r.PaymentAt = now.Add(-121 * 24 * time.Hour)
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "DISPUTE_WINDOW_EXPIRED", Message: "dispute window has expired"},
		},
		{
			name: "policy mismatch",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.Policy.Network = "mc"
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "DISPUTE_POLICY_MISMATCH", Message: "network policy does not match dispute network"},
		},
		{
			name: "missing policy",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.Policy = service.NetworkPolicy{}
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "DISPUTE_POLICY_REQUIRED", Message: "dispute requires a versioned network policy"},
		},
		{
			name: "missing hold",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.HoldID = ""
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "DISPUTE_HOLD_REQUIRED", Message: "dispute requires a durable hold reference"},
		},
		{
			name: "missing ids",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.DisputeID = ""
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "DISPUTE_ID_REQUIRED", Message: "dispute requires id, payment, and original posting"},
		},
		{
			name: "zero amount",
			req: mkReq(func(r *service.OpenDisputeRequest) {
				r.AmountMinor = 0
			}),
			expectedResult: entity.Dispute{},
			expectedError:  &entity.Error{Code: "INVALID_DISPUTE_AMOUNT", Message: "dispute amount must be positive"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.OpenDispute(tc.req)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestMarkEvidenceDue(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	open := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name           string
		d              entity.Dispute
		expectedResult entity.Dispute
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "open to due",
			d:    open,
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeEvidenceDue
				return d
			}(),
			expectedError: nil,
		},
		{
			name: "repeat rejected",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeEvidenceDue
				return d
			}(),
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeEvidenceDue
				return d
			}(),
			expectedError: &entity.Error{Code: "DISPUTE_STATUS_INVALID", Message: "dispute cannot await evidence in its current status"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.MarkEvidenceDue(tc.d)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSubmitEvidence(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	open := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name           string
		d              entity.Dispute
		at             time.Time
		expectedResult entity.Dispute
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "in window",
			d:    open,
			at:   now,
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			expectedError: nil,
		},
		{
			name:           "late",
			d:              open,
			at:             open.EvidenceDueAt.Add(time.Hour),
			expectedResult: open,
			expectedError:  &entity.Error{Code: "EVIDENCE_WINDOW_EXPIRED", Message: "dispute evidence window has expired"},
		},
		{
			name: "wrong status",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			at: now,
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			expectedError: &entity.Error{Code: "DISPUTE_STATUS_INVALID", Message: "dispute cannot accept evidence in its current status"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.SubmitEvidence(tc.d, tc.at)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestRepresentmentAllowed(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	policy := service.NetworkPolicy{
		Network:           "visa",
		Version:           "v2026.1",
		DisputeWindowDays: 120,
		EvidenceDays:      14,
		MaxRepresentments: 2,
		FeeMinor:          1500,
	}
	open := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name          string
		d             entity.Dispute
		policy        service.NetworkPolicy
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "allowed",
			d:             open,
			policy:        policy,
			expectedError: nil,
		},
		{
			name: "exhausted",
			d: func() entity.Dispute {
				d := open
				d.RepresentStage = 2
				return d
			}(),
			policy:        policy,
			expectedError: &entity.Error{Code: "REPRESENTMENT_EXHAUSTED", Message: "representment allowance is exhausted"},
		},
		{
			name: "version mismatch",
			d:    open,
			policy: func() service.NetworkPolicy {
				p := policy
				p.Version = "v2025.1"
				return p
			}(),
			expectedError: &entity.Error{Code: "DISPUTE_POLICY_MISMATCH", Message: "representment requires the dispute policy version"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := service.RepresentmentAllowed(tc.d, tc.policy)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCloseDispute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	open := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}
	type testCase struct {
		name           string
		d              entity.Dispute
		outcome        service.DisputeOutcome
		expectedResult entity.Dispute
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "lost",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			outcome: service.DisputeLost,
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeLost
				return d
			}(),
			expectedError: nil,
		},
		{
			name: "won",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			outcome: service.DisputeWon,
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeWon
				return d
			}(),
			expectedError: nil,
		},
		{
			name: "bad outcome",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			outcome: "MAYBE",
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeUnderReview
				return d
			}(),
			expectedError: &entity.Error{Code: "DISPUTE_OUTCOME_INVALID", Message: "dispute outcome must be WON or LOST"},
		},
		{
			name:           "open cannot close",
			d:              open,
			outcome:        service.DisputeLost,
			expectedResult: open,
			expectedError:  &entity.Error{Code: "DISPUTE_STATUS_INVALID", Message: "dispute cannot close in its current status"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.CloseDispute(tc.d, tc.outcome)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSealDispute(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	open := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name           string
		d              entity.Dispute
		expectedResult entity.Dispute
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "seal decided",
			d: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeLost
				return d
			}(),
			expectedResult: func() entity.Dispute {
				d := open
				d.Status = valueobject.DisputeClosed
				return d
			}(),
			expectedError: nil,
		},
		{
			name:           "seal undecided rejected",
			d:              open,
			expectedResult: open,
			expectedError:  &entity.Error{Code: "DISPUTE_STATUS_INVALID", Message: "only a decided dispute can be sealed"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.SealDispute(tc.d)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestEarlyWarningRecommend(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name                     string
		refundCostMinor          int64
		expectedDisputeCostMinor int64
		feeMinor                 int64
		expectedResult           bool
	}

	testCases := []testCase{
		{
			name:                     "cheap refund recommends",
			refundCostMinor:          100,
			expectedDisputeCostMinor: 500,
			feeMinor:                 1500,
			expectedResult:           true,
		},
		{
			name:                     "expensive refund stays",
			refundCostMinor:          10000,
			expectedDisputeCostMinor: 500,
			feeMinor:                 1500,
			expectedResult:           false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := service.EarlyWarningRecommend(tc.refundCostMinor, tc.expectedDisputeCostMinor, tc.feeMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestAuthorize(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	okAuth := service.Authorization{
		ID:               "a-1",
		AmountMinor:      10000,
		CapturedMinor:    0,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
		MultipleCaptures: false,
		RequiresAction:   false,
		HoldID:           "h-1",
	}

	type testCase struct {
		name             string
		id               string
		amountMinor      int64
		holdID           string
		now              time.Time
		expiryDays       int
		multipleCaptures bool
		expectedResult   service.Authorization
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "ok",
			id:               "a-1",
			amountMinor:      10000,
			holdID:           "h-1",
			now:              now,
			expiryDays:       7,
			multipleCaptures: false,
			expectedResult:   okAuth,
			expectedError:    nil,
		},
		{
			name:             "default expiry",
			id:               "a-1",
			amountMinor:      10000,
			holdID:           "h-1",
			now:              now,
			expiryDays:       0,
			multipleCaptures: false,
			expectedResult:   okAuth,
			expectedError:    nil,
		},
		{
			name:             "missing id",
			id:               "",
			amountMinor:      100,
			holdID:           "h",
			now:              now,
			expiryDays:       7,
			multipleCaptures: false,
			expectedResult:   service.Authorization{},
			expectedError:    &entity.Error{Code: "AUTH_ID_REQUIRED", Message: "authorization id is required"},
		},
		{
			name:             "zero amount",
			id:               "a",
			amountMinor:      0,
			holdID:           "h",
			now:              now,
			expiryDays:       7,
			multipleCaptures: false,
			expectedResult:   service.Authorization{},
			expectedError:    &entity.Error{Code: "INVALID_AUTH_AMOUNT", Message: "authorization amount must be positive"},
		},
		{
			name:             "missing hold",
			id:               "a",
			amountMinor:      100,
			holdID:           "",
			now:              now,
			expiryDays:       7,
			multipleCaptures: false,
			expectedResult:   service.Authorization{},
			expectedError:    &entity.Error{Code: "AUTH_HOLD_REQUIRED", Message: "authorization requires a hold reference"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.Authorize(tc.id, tc.amountMinor, tc.holdID, tc.now, tc.expiryDays, tc.multipleCaptures)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCapture(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	live := service.Authorization{
		ID:               "a-1",
		AmountMinor:      10000,
		CapturedMinor:    0,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
		MultipleCaptures: false,
		RequiresAction:   false,
		HoldID:           "h-1",
	}

	type testCase struct {
		name           string
		a              service.Authorization
		amountMinor    int64
		expectedResult service.Authorization
		expectedError  error
	}

	testCases := []testCase{
		{
			name:        "partial",
			a:           live,
			amountMinor: 6000,
			expectedResult: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			expectedError: nil,
		},
		{
			name:        "full capture",
			a:           live,
			amountMinor: 10000,
			expectedResult: func() service.Authorization {
				a := live
				a.CapturedMinor = 10000
				a.Status = valueobject.AuthCaptured
				return a
			}(),
			expectedError: nil,
		},
		{
			name: "over capture",
			a: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			amountMinor: 5000,
			expectedResult: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			expectedError: &entity.Error{Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount; remaining=4000"},
		},
		{
			name: "repeat partial forbidden",
			a: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			amountMinor: 1000,
			expectedResult: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			expectedError: &entity.Error{Code: "MULTIPLE_CAPTURES_FORBIDDEN", Message: "network forbids multiple partial captures"},
		},
		{
			name: "challenged not capturable",
			a: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthRequiresAction
				a.RequiresAction = true
				return a
			}(),
			amountMinor: 100,
			expectedResult: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthRequiresAction
				a.RequiresAction = true
				return a
			}(),
			expectedError: &entity.Error{Code: "AUTH_NOT_CAPTURABLE", Message: "authorization cannot capture in its current status"},
		},
		{
			name:           "zero amount",
			a:              live,
			amountMinor:    0,
			expectedResult: live,
			expectedError:  &entity.Error{Code: "INVALID_CAPTURE_AMOUNT", Message: "capture amount must be positive"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.Capture(tc.a, tc.amountMinor)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestSweepExpired(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	live := service.Authorization{
		ID:               "a-1",
		AmountMinor:      10000,
		CapturedMinor:    0,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
		MultipleCaptures: false,
		RequiresAction:   false,
		HoldID:           "h-1",
	}

	type testCase struct {
		name            string
		auths           []service.Authorization
		now             time.Time
		expectedResult1 []service.Authorization
		expectedResult2 []string
	}

	testCases := []testCase{
		{
			name: "expires once",
			auths: []service.Authorization{
				func() service.Authorization {
					a := live
					a.ExpiresAt = now.Add(-time.Hour)
					return a
				}(),
			},
			now: now,
			expectedResult1: []service.Authorization{
				func() service.Authorization {
					a := live
					a.ExpiresAt = now.Add(-time.Hour)
					a.Status = valueobject.AuthExpired
					return a
				}(),
			},
			expectedResult2: []string{"h-1"},
		},
		{
			name: "re-sweep idempotent",
			auths: []service.Authorization{
				func() service.Authorization {
					a := live
					a.ExpiresAt = now.Add(-time.Hour)
					a.Status = valueobject.AuthExpired
					return a
				}(),
			},
			now: now.Add(time.Hour),
			expectedResult1: []service.Authorization{
				func() service.Authorization {
					a := live
					a.ExpiresAt = now.Add(-time.Hour)
					a.Status = valueobject.AuthExpired
					return a
				}(),
			},
			expectedResult2: nil,
		},
		{
			name:            "live untouched",
			auths:           []service.Authorization{live},
			now:             now,
			expectedResult1: []service.Authorization{live},
			expectedResult2: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult1, actualResult2 := service.SweepExpired(tc.auths, tc.now)
			assert.Equal(t, tc.expectedResult1, actualResult1)
			assert.Equal(t, tc.expectedResult2, actualResult2)
		})
	}
}

func TestRequireAction(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	live := service.Authorization{
		ID:               "a-1",
		AmountMinor:      10000,
		CapturedMinor:    0,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
		MultipleCaptures: false,
		RequiresAction:   false,
		HoldID:           "h-1",
	}

	type testCase struct {
		name           string
		a              service.Authorization
		expectedResult service.Authorization
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "enter challenge",
			a:    live,
			expectedResult: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthRequiresAction
				a.RequiresAction = true
				return a
			}(),
			expectedError: nil,
		},
		{
			name: "partial cannot challenge",
			a: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			expectedResult: func() service.Authorization {
				a := live
				a.CapturedMinor = 6000
				a.Status = valueobject.AuthPartiallyCaptured
				return a
			}(),
			expectedError: &entity.Error{Code: "SCA_STATE_INVALID", Message: "challenge can start only from an authorized payment"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.RequireAction(tc.a)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestResolveAction(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	live := service.Authorization{
		ID:               "a-1",
		AmountMinor:      10000,
		CapturedMinor:    0,
		Status:           valueobject.AuthAuthorized,
		ExpiresAt:        now.Add(7 * 24 * time.Hour),
		MultipleCaptures: false,
		RequiresAction:   false,
		HoldID:           "h-1",
	}

	type testCase struct {
		name           string
		a              service.Authorization
		succeeded      bool
		expectedResult service.Authorization
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "resolve succeeded",
			a: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthRequiresAction
				a.RequiresAction = true
				return a
			}(),
			succeeded:      true,
			expectedResult: live,
			expectedError:  nil,
		},
		{
			name: "resolve failed",
			a: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthRequiresAction
				a.RequiresAction = true
				return a
			}(),
			succeeded: false,
			expectedResult: func() service.Authorization {
				a := live
				a.Status = valueobject.AuthVoided
				return a
			}(),
			expectedError: nil,
		},
		{
			name:           "no challenge",
			a:              live,
			succeeded:      true,
			expectedResult: live,
			expectedError:  &entity.Error{Code: "SCA_STATE_INVALID", Message: "no pending challenge to resolve"},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := service.ResolveAction(tc.a, tc.succeeded)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
