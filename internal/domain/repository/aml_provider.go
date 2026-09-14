package repository

import (
	"context"
)

// ScreeningDecision is the AML screening outcome.
type ScreeningDecision string

// Screening outcomes.
const (
	ScreenAllow  ScreeningDecision = "ALLOW"
	ScreenReview ScreeningDecision = "REVIEW"
	ScreenBlock  ScreeningDecision = "BLOCK"
)

// ScreeningRequest carries the transaction screening inputs.
type ScreeningRequest struct {
	TenantID    string
	AccountID   string
	AmountMinor int64
	AssetCode   string
	DayCount    int
	DaySumMinor int64
	Watchlisted bool
	RuleVersion string
}

// ReviewQueueEntry is the REVIEW hold-placement shape.
type ReviewQueueEntry struct {
	TenantID    string
	AccountID   string
	AmountMinor int64
	AssetCode   string
	ReasonCode  string
	RuleVersion string
}

// AMLProvider screens transactions before posting.
type AMLProvider interface {
	// Screen evaluates one transaction. Strong read; pure decision with no side effects.
	Screen(ctx context.Context, req ScreeningRequest) (ScreeningDecision, string, error)
}
