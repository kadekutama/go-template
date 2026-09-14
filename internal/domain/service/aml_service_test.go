package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/service"
)

func TestScreenTransaction(t *testing.T) {
	t.Parallel()

	baseReq := repository.ScreeningRequest{
		TenantID:    "t-1",
		AccountID:   "a-1",
		AmountMinor: 1000,
		AssetCode:   "USD",
		DayCount:    2,
		DaySumMinor: 3000,
		Watchlisted: false,
		RuleVersion: "v1",
	}
	basePolicy := service.ScreeningPolicy{
		MaxAmountMinor: 10000,
		MaxDayCount:    5,
		MaxDaySumMinor: 20000,
		RuleVersion:    "v1",
	}

	type testCase struct {
		name             string
		req              repository.ScreeningRequest
		policy           service.ScreeningPolicy
		expectedDecision repository.ScreeningDecision
		expectedReason   string
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "allow under thresholds",
			req:              baseReq,
			policy:           basePolicy,
			expectedDecision: repository.ScreenAllow,
			expectedReason:   "OK",
			expectedError:    nil,
		},
		{
			name: "watchlist blocks below thresholds",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.Watchlisted = true
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "WATCHLIST_HIT",
			expectedError:    nil,
		},
		{
			name: "watchlist precedence over amount threshold",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.Watchlisted = true
				r.AmountMinor = 50000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "WATCHLIST_HIT",
			expectedError:    nil,
		},
		{
			name: "watchlist precedence over velocity count",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.Watchlisted = true
				r.DayCount = 20
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "WATCHLIST_HIT",
			expectedError:    nil,
		},
		{
			name: "watchlist precedence over velocity sum",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.Watchlisted = true
				r.DaySumMinor = 100000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "WATCHLIST_HIT",
			expectedError:    nil,
		},
		{
			name: "amount over max blocks",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 20000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "amount precedence over velocity count",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 20000
				r.DayCount = 20
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "amount precedence over velocity sum",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 20000
				r.DaySumMinor = 50000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "velocity count over max reviews",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DayCount = 10
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenReview,
			expectedReason:   "VELOCITY_COUNT_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "velocity count precedence over velocity sum",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DayCount = 10
				r.DaySumMinor = 50000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenReview,
			expectedReason:   "VELOCITY_COUNT_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "velocity sum over max reviews",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DaySumMinor = 50000
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenReview,
			expectedReason:   "VELOCITY_SUM_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "at max boundaries allows",
			req: repository.ScreeningRequest{
				TenantID:    "t-1",
				AccountID:   "a-1",
				AmountMinor: 10000,
				AssetCode:   "USD",
				DayCount:    5,
				DaySumMinor: 20000,
				Watchlisted: false,
				RuleVersion: "v1",
			},
			policy:           basePolicy,
			expectedDecision: repository.ScreenAllow,
			expectedReason:   "OK",
			expectedError:    nil,
		},
		{
			name: "one over max amount blocks",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 10001
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "one over max day count reviews",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DayCount = 6
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenReview,
			expectedReason:   "VELOCITY_COUNT_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "one over max day sum reviews",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DaySumMinor = 20001
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenReview,
			expectedReason:   "VELOCITY_SUM_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "max int64 amount blocks without overflow",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 9223372036854775807
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "zero amount rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = 0
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("INVALID_ENTRY_AMOUNT", "screening amount must be positive"),
		},
		{
			name: "negative amount rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AmountMinor = -100
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("INVALID_ENTRY_AMOUNT", "screening amount must be positive"),
		},
		{
			name: "missing tenant rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids"),
		},
		{
			name: "whitespace tenant rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.TenantID = "   "
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids"),
		},
		{
			name: "missing account rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AccountID = ""
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids"),
		},
		{
			name: "whitespace account rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AccountID = "   "
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids"),
		},
		{
			name: "missing asset rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AssetCode = "  "
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_ASSET_REQUIRED", "screening asset is required"),
		},
		{
			name: "missing req rule version rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.RuleVersion = ""
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("RULE_VERSION_REQUIRED", "screening requires a rule version"),
		},
		{
			name: "whitespace req rule version rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.RuleVersion = "   "
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("RULE_VERSION_REQUIRED", "screening requires a rule version"),
		},
		{
			name: "missing policy version rejected",
			req:  baseReq,
			policy: func() service.ScreeningPolicy {
				p := basePolicy
				p.RuleVersion = ""
				return p
			}(),
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("RULE_VERSION_REQUIRED", "screening requires a rule version"),
		},
		{
			name: "whitespace policy version rejected",
			req:  baseReq,
			policy: func() service.ScreeningPolicy {
				p := basePolicy
				p.RuleVersion = "   "
				return p
			}(),
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("RULE_VERSION_REQUIRED", "screening requires a rule version"),
		},
		{
			name: "negative policy max amount rejected",
			req:  baseReq,
			policy: func() service.ScreeningPolicy {
				p := basePolicy
				p.MaxAmountMinor = -1
				return p
			}(),
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_POLICY_INVALID", "screening thresholds must be non-negative"),
		},
		{
			name: "negative policy max day count rejected",
			req:  baseReq,
			policy: func() service.ScreeningPolicy {
				p := basePolicy
				p.MaxDayCount = -1
				return p
			}(),
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_POLICY_INVALID", "screening thresholds must be non-negative"),
		},
		{
			name: "negative policy max day sum rejected",
			req:  baseReq,
			policy: func() service.ScreeningPolicy {
				p := basePolicy
				p.MaxDaySumMinor = -1
				return p
			}(),
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_POLICY_INVALID", "screening thresholds must be non-negative"),
		},
		{
			name: "negative day count rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DayCount = -1
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_VELOCITY_INVALID", "velocity inputs must be non-negative"),
		},
		{
			name: "negative day sum rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.DaySumMinor = -5
				return r
			}(),
			policy:           basePolicy,
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_VELOCITY_INVALID", "velocity inputs must be non-negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, reason, err := service.ScreenTransaction(tc.req, tc.policy)
			assert.Equal(t, tc.expectedDecision, actualResult)
			assert.Equal(t, tc.expectedReason, reason)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBuildReviewEntry(t *testing.T) {
	t.Parallel()

	baseReq := repository.ScreeningRequest{
		TenantID:    "t-1",
		AccountID:   "a-1",
		AmountMinor: 5000,
		AssetCode:   "USD",
		RuleVersion: "v1",
	}

	type testCase struct {
		name           string
		req            repository.ScreeningRequest
		reasonCode     string
		expectedResult repository.ReviewQueueEntry
		expectedError  error
	}

	testCases := []testCase{
		{
			name:       "valid review entry",
			req:        baseReq,
			reasonCode: "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{
				TenantID:    "t-1",
				AccountID:   "a-1",
				AmountMinor: 5000,
				AssetCode:   "USD",
				ReasonCode:  "VELOCITY_COUNT_EXCEEDED",
				RuleVersion: "v1",
			},
			expectedError: nil,
		},
		{
			name:           "missing reason rejected",
			req:            baseReq,
			reasonCode:     "",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_REASON_REQUIRED", "review entry requires a reason code"),
		},
		{
			name:           "whitespace reason rejected",
			req:            baseReq,
			reasonCode:     "   ",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_REASON_REQUIRED", "review entry requires a reason code"),
		},
		{
			name: "missing tenant rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.TenantID = ""
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_IDENTITY_REQUIRED", "review entry requires tenant and account ids"),
		},
		{
			name: "whitespace tenant rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.TenantID = "   "
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_IDENTITY_REQUIRED", "review entry requires tenant and account ids"),
		},
		{
			name: "missing account rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AccountID = ""
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_IDENTITY_REQUIRED", "review entry requires tenant and account ids"),
		},
		{
			name: "whitespace account rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.AccountID = "   "
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("SCREENING_IDENTITY_REQUIRED", "review entry requires tenant and account ids"),
		},
		{
			name: "missing rule version rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.RuleVersion = ""
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("RULE_VERSION_REQUIRED", "review entry requires a rule version"),
		},
		{
			name: "whitespace rule version rejected",
			req: func() repository.ScreeningRequest {
				r := baseReq
				r.RuleVersion = "   "
				return r
			}(),
			reasonCode:     "VELOCITY_COUNT_EXCEEDED",
			expectedResult: repository.ReviewQueueEntry{},
			expectedError:  entity.NewError("RULE_VERSION_REQUIRED", "review entry requires a rule version"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.BuildReviewEntry(tc.req, tc.reasonCode)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestScreenViaProviderPort(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		ctx              context.Context
		req              repository.ScreeningRequest
		expectedDecision repository.ScreeningDecision
		expectedReason   string
		expectedError    error
	}

	testCases := []testCase{
		{
			name: "port allows under thresholds",
			ctx:  context.Background(),
			req: repository.ScreeningRequest{
				TenantID:    "t-1",
				AccountID:   "a-1",
				AmountMinor: 100,
				AssetCode:   "USD",
				RuleVersion: "v1",
			},
			expectedDecision: repository.ScreenAllow,
			expectedReason:   "OK",
			expectedError:    nil,
		},
		{
			name: "port blocks over amount",
			ctx:  context.Background(),
			req: repository.ScreeningRequest{
				TenantID:    "t-1",
				AccountID:   "a-1",
				AmountMinor: 5000,
				AssetCode:   "USD",
				RuleVersion: "v1",
			},
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "AMOUNT_THRESHOLD_EXCEEDED",
			expectedError:    nil,
		},
		{
			name: "port blocks watchlisted actor",
			ctx:  context.Background(),
			req: repository.ScreeningRequest{
				TenantID:    "t-1",
				AccountID:   "a-1",
				AmountMinor: 100,
				AssetCode:   "USD",
				Watchlisted: true,
				RuleVersion: "v1",
			},
			expectedDecision: repository.ScreenBlock,
			expectedReason:   "WATCHLIST_HIT",
			expectedError:    nil,
		},
		{
			name: "port rejects missing identity",
			ctx:  context.Background(),
			req: repository.ScreeningRequest{
				TenantID:    "",
				AccountID:   "a-1",
				AmountMinor: 100,
				AssetCode:   "USD",
				RuleVersion: "v1",
			},
			expectedDecision: repository.ScreeningDecision(""),
			expectedReason:   "",
			expectedError:    entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider := fakeAMLProvider{}
			actualDecision, reason, err := provider.Screen(tc.ctx, tc.req)
			assert.Equal(t, tc.expectedDecision, actualDecision)
			assert.Equal(t, tc.expectedReason, reason)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

type fakeAMLProvider struct{}

func (fakeAMLProvider) Screen(_ context.Context, req repository.ScreeningRequest) (repository.ScreeningDecision, string, error) {
	return service.ScreenTransaction(req, service.ScreeningPolicy{MaxAmountMinor: 1000, MaxDayCount: 10, MaxDaySumMinor: 10000, RuleVersion: "v1"})
}
