package service

import (
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ResolutionRequest is a break resolution command. Times are explicit params.
type ResolutionRequest struct {
	BreakID             string
	Action              valueobject.ResolutionAction
	ReasonCode          string
	EvidenceRef         string
	Owner               string
	Actor               string
	Approver            string
	AuditLink           string
	AdjustmentPostingID string
	AmountMinor         int64
	AssetCode           string
	ExpiresAt           time.Time
	Now                 time.Time
}

// AdjustmentLine is one side of a linked adjustment posting.
type AdjustmentLine struct {
	AccountID   string
	Side        string
	AmountMinor int64
	AssetCode   string
}

// ValidateResolution enforces evidence, ownership, and SoD before transition.
func ValidateResolution(req ResolutionRequest, current valueobject.BreakStatus, thresholdMinor int64) error {
	if err := validateResolutionIdentity(req); err != nil {
		return err
	}
	if err := validateResolutionTransition(req, current); err != nil {
		return err
	}
	if err := validateResolutionAmounts(req, thresholdMinor); err != nil {
		return err
	}
	return validateAcknowledgement(req)
}

func validateResolutionIdentity(req ResolutionRequest) error {
	if strings.TrimSpace(req.BreakID) == "" {
		return entity.NewError("BREAK_ID_REQUIRED", "break id is required")
	}
	if _, err := valueobject.ParseResolutionAction(string(req.Action)); err != nil {
		return entity.NewError("RESOLUTION_ACTION_INVALID", "resolution action is invalid")
	}
	if strings.TrimSpace(req.ReasonCode) == "" || strings.TrimSpace(req.EvidenceRef) == "" || strings.TrimSpace(req.Owner) == "" || strings.TrimSpace(req.Actor) == "" || strings.TrimSpace(req.AuditLink) == "" {
		return entity.NewError("EVIDENCE_REQUIRED", "resolution requires reason, evidence, owner, actor, and audit link")
	}
	if req.Now.IsZero() {
		return entity.NewError("RESOLUTION_TIME_REQUIRED", "resolution time is required")
	}
	return nil
}

func validateResolutionTransition(req ResolutionRequest, current valueobject.BreakStatus) error {
	if !valueobject.CanTransitionBreak(current, req.Action) {
		return entity.NewError("ILLEGAL_TRANSITION", "resolution action is not allowed from current status")
	}
	return nil
}

func validateAcknowledgement(req ResolutionRequest) error {
	if req.Action != valueobject.ResolveAcknowledge {
		return nil
	}
	if req.ExpiresAt.IsZero() {
		return entity.NewError("EVIDENCE_REQUIRED", "acknowledgement requires an expiry")
	}
	if !req.ExpiresAt.After(req.Now) {
		return entity.NewError("ACK_EXPIRY_INVALID", "acknowledgement expiry must be in the future")
	}
	return nil
}

// TransitionBreak moves a break one step. OPEN + action enters IN_REVIEW;
// IN_REVIEW resolves/acknowledges/escalates; ESCALATED may still resolve.
func TransitionBreak(current valueobject.BreakStatus, action valueobject.ResolutionAction) (valueobject.BreakStatus, error) {
	switch current {
	case valueobject.BreakOpen:
		switch action {
		case valueobject.ResolveAcknowledge, valueobject.ResolveEscalate, valueobject.ResolveExternalError, valueobject.ResolveAdjustLedger:
			return valueobject.BreakInReview, nil
		default:
			return "", entity.NewError("ILLEGAL_TRANSITION", "unknown resolution action")
		}
	case valueobject.BreakInReview:
		switch action {
		case valueobject.ResolveAdjustLedger, valueobject.ResolveExternalError:
			return valueobject.BreakResolved, nil
		case valueobject.ResolveAcknowledge:
			return valueobject.BreakAcknowledged, nil
		case valueobject.ResolveEscalate:
			return valueobject.BreakEscalated, nil
		default:
			return "", entity.NewError("ILLEGAL_TRANSITION", "unknown resolution action")
		}
	case valueobject.BreakEscalated:
		switch action {
		case valueobject.ResolveAdjustLedger, valueobject.ResolveExternalError:
			return valueobject.BreakResolved, nil
		default:
			return "", entity.NewError("ILLEGAL_TRANSITION", "escalated breaks only resolve via adjustment or external error")
		}
	default:
		return "", entity.NewError("ILLEGAL_TRANSITION", "break cannot transition from its current status")
	}
}

// IsAcknowledgementExpired reports whether an acknowledgement has lapsed.
func IsAcknowledgementExpired(expiresAt, now time.Time) bool {
	if expiresAt.IsZero() || now.IsZero() {
		return false
	}
	return now.After(expiresAt)
}

// ReopenIfExpired re-opens an expired acknowledgement, completing the
// ACKNOWLEDGED (expiry → re-OPEN) leg of the state machine.
func ReopenIfExpired(current valueobject.BreakStatus, expiresAt, now time.Time) (valueobject.BreakStatus, error) {
	if current != valueobject.BreakAcknowledged {
		return "", entity.NewError("ILLEGAL_TRANSITION", "only acknowledged breaks reopen on expiry")
	}
	if !IsAcknowledgementExpired(expiresAt, now) {
		return "", entity.NewError("ACK_ACTIVE", "acknowledgement has not expired")
	}
	return valueobject.BreakOpen, nil
}

// BuildAdjustmentLines constructs the balanced linked adjustment.
func BuildAdjustmentLines(breakID, originalPostingID, debitAccount, creditAccount string, amountMinor int64, assetCode string) ([2]AdjustmentLine, error) {
	if strings.TrimSpace(breakID) == "" || strings.TrimSpace(originalPostingID) == "" {
		return [2]AdjustmentLine{}, entity.NewError("BREAK_ID_REQUIRED", "adjustment requires break and original posting ids")
	}
	debit := strings.TrimSpace(debitAccount)
	credit := strings.TrimSpace(creditAccount)
	if debit == "" || credit == "" {
		return [2]AdjustmentLine{}, entity.NewError("ADJUSTMENT_ACCOUNT_REQUIRED", "adjustment requires debit and credit accounts")
	}
	if debit == credit {
		return [2]AdjustmentLine{}, entity.NewError("ADJUSTMENT_UNBALANCED", "adjustment debit and credit must differ")
	}
	if amountMinor <= 0 {
		return [2]AdjustmentLine{}, entity.NewError("INVALID_ENTRY_AMOUNT", "adjustment amount must be positive")
	}
	if strings.TrimSpace(assetCode) == "" {
		return [2]AdjustmentLine{}, entity.NewError("ADJUSTMENT_ASSET_REQUIRED", "adjustment asset is required")
	}
	return [2]AdjustmentLine{
		{AccountID: debit, Side: "DEBIT", AmountMinor: amountMinor, AssetCode: assetCode},
		{AccountID: credit, Side: "CREDIT", AmountMinor: amountMinor, AssetCode: assetCode},
	}, nil
}

func validateResolutionAmounts(req ResolutionRequest, thresholdMinor int64) error {
	if req.Action != valueobject.ResolveAdjustLedger {
		return nil
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_ENTRY_AMOUNT", "adjustment amount must be positive")
	}
	if strings.TrimSpace(req.AdjustmentPostingID) == "" {
		return entity.NewError("EVIDENCE_REQUIRED", "adjustment requires the linked posting id")
	}
	if thresholdMinor < 0 {
		return entity.NewError("THRESHOLD_INVALID", "approval threshold must be non-negative")
	}
	if req.AmountMinor > thresholdMinor {
		if strings.TrimSpace(req.Approver) == "" {
			return entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment above threshold requires an approver")
		}
		if req.Approver == req.Actor {
			return entity.NewError("SELF_APPROVAL_FORBIDDEN", "adjustment approver must differ from maker")
		}
	}
	return nil
}
