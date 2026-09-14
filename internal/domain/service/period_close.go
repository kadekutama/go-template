package service

import (
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// CloseInput carries the period-close gate inputs.
type CloseInput struct {
	Period                  entity.PeriodData
	UnresolvedWorkflowCount int
	OpenBreakCount          int
	SubledgerDeltas         map[string]int64
	FXRevalued              bool
}

// ClosingLine is one side of the income-summary closing entry.
type ClosingLine struct {
	AccountID   string
	Side        string
	AmountMinor int64
	AssetCode   string
}

// ValidateClose returns one error per failing gate check.
func ValidateClose(in CloseInput) []error {
	var errs []error
	if in.Period.Status != entity.PeriodOpen {
		errs = append(errs, entity.NewError("PERIOD_CLOSED", "period close requires an open period"))
	}
	if in.UnresolvedWorkflowCount < 0 || in.OpenBreakCount < 0 {
		errs = append(errs, entity.NewError("CLOSE_INPUT_INVALID", "workflow and break counts must be non-negative"))
	} else {
		if in.UnresolvedWorkflowCount > 0 {
			errs = append(errs, entity.NewError("WORKFLOWS_PENDING", "period has unresolved workflows"))
		}
		if in.OpenBreakCount > 0 {
			errs = append(errs, entity.NewError("BREAKS_OPEN", "period has open reconciliation breaks"))
		}
	}
	if hasSubledgerDelta(in.SubledgerDeltas) {
		errs = append(errs, entity.NewError("SUBLEDGERS_UNBALANCED", "sub-ledgers do not balance"))
	}
	if !in.FXRevalued {
		errs = append(errs, entity.NewError("FX_NOT_REVALUED", "foreign balances are not revalued"))
	}
	return errs
}

// BuildClosingEntries constructs the balanced income-summary entry.
func BuildClosingEntries(incomeSummaryAccount, retainedEarningsAccount string, amountMinor int64, assetCode string) ([2]ClosingLine, error) {
	income := strings.TrimSpace(incomeSummaryAccount)
	retained := strings.TrimSpace(retainedEarningsAccount)
	if income == "" || retained == "" {
		return [2]ClosingLine{}, entity.NewError("CLOSING_ACCOUNT_REQUIRED", "closing requires income summary and retained earnings accounts")
	}
	if income == retained {
		return [2]ClosingLine{}, entity.NewError("CLOSING_UNBALANCED", "closing accounts must differ")
	}
	if amountMinor <= 0 {
		return [2]ClosingLine{}, entity.NewError("INVALID_ENTRY_AMOUNT", "closing amount must be positive")
	}
	asset := strings.TrimSpace(assetCode)
	if asset == "" {
		return [2]ClosingLine{}, entity.NewError("CLOSING_ASSET_REQUIRED", "closing asset is required")
	}
	return [2]ClosingLine{
		{AccountID: income, Side: "DEBIT", AmountMinor: amountMinor, AssetCode: asset},
		{AccountID: retained, Side: "CREDIT", AmountMinor: amountMinor, AssetCode: asset},
	}, nil
}

// ValidateReopen enforces privileged maker-checker reopen.
func ValidateReopen(maker, checker, reason string) error {
	m := strings.TrimSpace(maker)
	c := strings.TrimSpace(checker)
	r := strings.TrimSpace(reason)
	if r == "" {
		return entity.NewError("REOPEN_REASON_REQUIRED", "period reopen requires a reason")
	}
	if m == "" || c == "" {
		return entity.NewError("REOPEN_APPROVER_REQUIRED", "period reopen requires maker and checker")
	}
	if m == c {
		return entity.NewError("SELF_APPROVAL_FORBIDDEN", "period reopen checker must differ from maker")
	}
	return nil
}

// ValidateLateCorrection enforces next-open-period corrections.
func ValidateLateCorrection(originalEffectiveAt time.Time, targetPeriod entity.PeriodData, originalDateRetained bool) error {
	if originalEffectiveAt.IsZero() {
		return entity.NewError("CORRECTION_DATE_REQUIRED", "late correction requires the original effective date")
	}
	if targetPeriod.Status != entity.PeriodOpen {
		return entity.NewError("LATE_CORRECTION_PERIOD_REQUIRED", "late corrections must target the next open period")
	}
	if !originalDateRetained {
		return entity.NewError("CORRECTION_CONTEXT_REQUIRED", "late correction must retain original effective-date context")
	}
	if !targetPeriod.Contains(originalEffectiveAt) && !originalEffectiveAt.Before(targetPeriod.Start) {
		return entity.NewError("LATE_CORRECTION_PERIOD_REQUIRED", "late correction target must follow the original date")
	}
	return nil
}

func hasSubledgerDelta(deltas map[string]int64) bool {
	for _, v := range deltas {
		if v != 0 {
			return true
		}
	}
	return false
}
