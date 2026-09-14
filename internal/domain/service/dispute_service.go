package service

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// NetworkPolicy is the versioned network configuration: windows, evidence
// period, representment allowance, and fee. Never hard-code "exactly once".
type NetworkPolicy struct {
	Network           string
	Version           string
	DisputeWindowDays int
	EvidenceDays      int
	MaxRepresentments int
	FeeMinor          int64
}

// OpenDisputeRequest opens a dispute against a payment.
type OpenDisputeRequest struct {
	DisputeID         string
	PaymentID         string
	OriginalPostingID string
	Network           string
	AmountMinor       int64
	PaymentAt         time.Time
	Now               time.Time
	HoldID            string
	Policy            NetworkPolicy
}

// OpenDispute validates the network window and builds the dispute with its
// evidence deadline, durable hold reference, and fee.
func OpenDispute(req OpenDisputeRequest) (entity.Dispute, error) {
	if req.DisputeID == "" || req.PaymentID == "" || req.OriginalPostingID == "" {
		return entity.Dispute{}, entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting")
	}
	if req.AmountMinor <= 0 {
		return entity.Dispute{}, entity.NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive")
	}
	if req.Policy.Network == "" || req.Policy.Version == "" {
		return entity.Dispute{}, entity.NewError("DISPUTE_POLICY_REQUIRED", "dispute requires a versioned network policy")
	}
	if req.Policy.Network != req.Network {
		return entity.Dispute{}, entity.NewError("DISPUTE_POLICY_MISMATCH", "network policy does not match dispute network")
	}
	if req.HoldID == "" {
		return entity.Dispute{}, entity.NewError("DISPUTE_HOLD_REQUIRED", "dispute requires a durable hold reference")
	}
	window := time.Duration(req.Policy.DisputeWindowDays) * 24 * time.Hour
	if req.Now.Sub(req.PaymentAt) > window {
		return entity.Dispute{}, entity.NewError("DISPUTE_WINDOW_EXPIRED", "dispute window has expired")
	}
	d := entity.Dispute{
		ID:                req.DisputeID,
		PaymentID:         req.PaymentID,
		OriginalPostingID: req.OriginalPostingID,
		Network:           req.Network,
		AmountMinor:       req.AmountMinor,
		OpenedAt:          req.Now,
		EvidenceDueAt:     req.Now.Add(time.Duration(req.Policy.EvidenceDays) * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            req.HoldID,
		FeeMinor:          req.Policy.FeeMinor,
		RepresentStage:    0,
		PolicyVersion:     req.Policy.Version,
	}
	if err := d.Validate(); err != nil {
		return entity.Dispute{}, err
	}
	return d, nil
}

// MarkEvidenceDue moves an OPEN dispute to EVIDENCE_DUE (evidence awaited).
func MarkEvidenceDue(d entity.Dispute) (entity.Dispute, error) {
	if !valueobject.CanTransitionDispute(d.Status, valueobject.DisputeEvidenceDue) {
		return d, entity.NewError("DISPUTE_STATUS_INVALID", "dispute cannot await evidence in its current status")
	}
	d.Status = valueobject.DisputeEvidenceDue
	return d, nil
}

// SubmitEvidence enforces the evidence window and the transition table.
func SubmitEvidence(d entity.Dispute, at time.Time) (entity.Dispute, error) {
	if at.After(d.EvidenceDueAt) {
		return d, entity.NewError("EVIDENCE_WINDOW_EXPIRED", "dispute evidence window has expired")
	}
	if !valueobject.CanTransitionDispute(d.Status, valueobject.DisputeUnderReview) {
		return d, entity.NewError("DISPUTE_STATUS_INVALID", "dispute cannot accept evidence in its current status")
	}
	d.Status = valueobject.DisputeUnderReview
	return d, nil
}

// RepresentmentAllowed enforces versioned representment stages/count.
func RepresentmentAllowed(d entity.Dispute, policy NetworkPolicy) error {
	if d.PolicyVersion != policy.Version {
		return entity.NewError("DISPUTE_POLICY_MISMATCH", "representment requires the dispute policy version")
	}
	if d.RepresentStage >= policy.MaxRepresentments {
		return entity.NewError("REPRESENTMENT_EXHAUSTED", "representment allowance is exhausted")
	}
	return nil
}

// DisputeOutcome is won or lost.
type DisputeOutcome string

// Dispute outcomes.
const (
	DisputeWon  DisputeOutcome = "WON"
	DisputeLost DisputeOutcome = "LOST"
)

// CloseDispute resolves the dispute: won releases the hold, lost consumes it
// and links a balanced reversal to the original posting. Only disputes under
// review may close.
func CloseDispute(d entity.Dispute, outcome DisputeOutcome) (entity.Dispute, error) {
	var target valueobject.DisputeStatus
	switch outcome {
	case DisputeWon:
		target = valueobject.DisputeWon
	case DisputeLost:
		target = valueobject.DisputeLost
	default:
		return d, entity.NewError("DISPUTE_OUTCOME_INVALID", "dispute outcome must be WON or LOST")
	}
	if !valueobject.CanTransitionDispute(d.Status, target) {
		return d, entity.NewError("DISPUTE_STATUS_INVALID", "dispute cannot close in its current status")
	}
	d.Status = target
	return d, nil
}

// SealDispute archives a decided dispute. Terminal: no outgoing moves.
func SealDispute(d entity.Dispute) (entity.Dispute, error) {
	if !valueobject.CanTransitionDispute(d.Status, valueobject.DisputeClosed) {
		return d, entity.NewError("DISPUTE_STATUS_INVALID", "only a decided dispute can be sealed")
	}
	d.Status = valueobject.DisputeClosed
	return d, nil
}

// EarlyWarningRecommend recommends auto-refund when refund cost is below the
// expected dispute cost plus fee.
func EarlyWarningRecommend(refundCostMinor, expectedDisputeCostMinor, feeMinor int64) bool {
	return refundCostMinor < expectedDisputeCostMinor+feeMinor
}
