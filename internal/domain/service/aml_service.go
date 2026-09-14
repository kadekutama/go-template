package service

import (
	"strings"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
)

// ScreeningPolicy carries versioned thresholds.
type ScreeningPolicy struct {
	MaxAmountMinor int64
	MaxDayCount    int
	MaxDaySumMinor int64
	RuleVersion    string
}

// ScreenTransaction evaluates thresholds, velocity, and watchlist in order.
func ScreenTransaction(req repository.ScreeningRequest, policy ScreeningPolicy) (repository.ScreeningDecision, string, error) {
	if err := validateScreeningInputs(req, policy); err != nil {
		return "", "", err
	}
	if req.Watchlisted {
		return repository.ScreenBlock, "WATCHLIST_HIT", nil
	}
	if req.AmountMinor > policy.MaxAmountMinor {
		return repository.ScreenBlock, "AMOUNT_THRESHOLD_EXCEEDED", nil
	}
	if req.DayCount > policy.MaxDayCount {
		return repository.ScreenReview, "VELOCITY_COUNT_EXCEEDED", nil
	}
	if req.DaySumMinor > policy.MaxDaySumMinor {
		return repository.ScreenReview, "VELOCITY_SUM_EXCEEDED", nil
	}
	return repository.ScreenAllow, "OK", nil
}

// BuildReviewEntry constructs the REVIEW queue shape for hold placement.
func BuildReviewEntry(req repository.ScreeningRequest, reasonCode string) (repository.ReviewQueueEntry, error) {
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.AccountID) == "" {
		return repository.ReviewQueueEntry{}, entity.NewError("SCREENING_IDENTITY_REQUIRED", "review entry requires tenant and account ids")
	}
	if strings.TrimSpace(reasonCode) == "" {
		return repository.ReviewQueueEntry{}, entity.NewError("SCREENING_REASON_REQUIRED", "review entry requires a reason code")
	}
	if strings.TrimSpace(req.RuleVersion) == "" {
		return repository.ReviewQueueEntry{}, entity.NewError("RULE_VERSION_REQUIRED", "review entry requires a rule version")
	}
	return repository.ReviewQueueEntry{
		TenantID:    req.TenantID,
		AccountID:   req.AccountID,
		AmountMinor: req.AmountMinor,
		AssetCode:   req.AssetCode,
		ReasonCode:  reasonCode,
		RuleVersion: req.RuleVersion,
	}, nil
}

func validateScreeningInputs(req repository.ScreeningRequest, policy ScreeningPolicy) error {
	if strings.TrimSpace(req.TenantID) == "" || strings.TrimSpace(req.AccountID) == "" {
		return entity.NewError("SCREENING_IDENTITY_REQUIRED", "screening requires tenant and account ids")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_ENTRY_AMOUNT", "screening amount must be positive")
	}
	if strings.TrimSpace(req.AssetCode) == "" {
		return entity.NewError("SCREENING_ASSET_REQUIRED", "screening asset is required")
	}
	if strings.TrimSpace(req.RuleVersion) == "" || strings.TrimSpace(policy.RuleVersion) == "" {
		return entity.NewError("RULE_VERSION_REQUIRED", "screening requires a rule version")
	}
	if policy.MaxAmountMinor < 0 || policy.MaxDayCount < 0 || policy.MaxDaySumMinor < 0 {
		return entity.NewError("SCREENING_POLICY_INVALID", "screening thresholds must be non-negative")
	}
	if req.DayCount < 0 || req.DaySumMinor < 0 {
		return entity.NewError("SCREENING_VELOCITY_INVALID", "velocity inputs must be non-negative")
	}
	return nil
}
