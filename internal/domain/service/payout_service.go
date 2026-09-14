package service

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PayoutReference carries provider traceability for a payout.
type PayoutReference struct {
	ProviderReference string
	TraceID           string
	IdempotencyKey    string
}

// PayoutSubmission is the staged submission journal shape (ledger-core §6.3):
// DR Merchant Payable (available) / CR Payouts Payable.
type PayoutSubmission struct {
	MerchantAccount valueobject.AccountID
	PayoutsPayable  valueobject.AccountID
	AmountMinor     int64
	AssetCode       valueobject.AssetCode
	Method          valueobject.PayoutMethod
	Reference       PayoutReference
}

// PayoutSettlement is the bank-settlement shape: DR Payouts Payable /
// CR Bank Cash.
type PayoutSettlement struct {
	PayoutsPayable valueobject.AccountID
	BankCash       valueobject.AccountID
	AmountMinor    int64
	AssetCode      valueobject.AssetCode
	Method         valueobject.PayoutMethod
	SettledAt      time.Time
}

// ValidateSubmission checks payout submission inputs and builds the staged
// submission lines.
func ValidateSubmission(sub PayoutSubmission) (PayoutSubmission, error) {
	if sub.AmountMinor <= 0 {
		return PayoutSubmission{}, entity.NewError("INVALID_PAYOUT_AMOUNT", "payout amount must be positive")
	}
	if _, err := valueobject.ParsePayoutMethod(string(sub.Method)); err != nil {
		return PayoutSubmission{}, entity.NewError("PAYOUT_METHOD_UNKNOWN", "payout method is unknown")
	}
	if sub.MerchantAccount.String() == "" || sub.PayoutsPayable.String() == "" {
		return PayoutSubmission{}, entity.NewError("PAYOUT_ACCOUNT_REQUIRED", "payout requires merchant and payouts-payable accounts")
	}
	if sub.MerchantAccount == sub.PayoutsPayable {
		return PayoutSubmission{}, entity.NewError("PAYOUT_ACCOUNT_INVALID", "payout staged accounts must be distinct")
	}
	if sub.AssetCode == "" {
		return PayoutSubmission{}, entity.NewError("PAYOUT_ASSET_REQUIRED", "payout requires an asset code")
	}
	if sub.Reference.IdempotencyKey == "" {
		return PayoutSubmission{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "payout requires an idempotency key")
	}
	return sub, nil
}

// ValidateSettlement checks the settlement leg against the submission.
func ValidateSettlement(sub PayoutSubmission, stl PayoutSettlement) (PayoutSettlement, error) {
	if stl.Method != sub.Method {
		return PayoutSettlement{}, entity.NewError("PAYOUT_SETTLEMENT_MISMATCH", "settlement must use the submission rail")
	}
	if stl.AmountMinor != sub.AmountMinor || stl.AssetCode != sub.AssetCode {
		return PayoutSettlement{}, entity.NewError("PAYOUT_SETTLEMENT_MISMATCH", "settlement must match submission amount and asset")
	}
	if stl.PayoutsPayable != sub.PayoutsPayable {
		return PayoutSettlement{}, entity.NewError("PAYOUT_SETTLEMENT_MISMATCH", "settlement must clear the submission payable")
	}
	if stl.BankCash.String() == "" {
		return PayoutSettlement{}, entity.NewError("PAYOUT_ACCOUNT_REQUIRED", "settlement requires a bank cash account")
	}
	if stl.BankCash == stl.PayoutsPayable {
		return PayoutSettlement{}, entity.NewError("PAYOUT_ACCOUNT_INVALID", "settlement accounts must be distinct")
	}
	if stl.SettledAt.IsZero() {
		return PayoutSettlement{}, entity.NewError("SETTLED_AT_REQUIRED", "settlement requires a settlement timestamp")
	}
	return stl, nil
}

// CancelPayout enforces cancel-only-while-PENDING.
func CancelPayout(status valueobject.PayoutStatus) (valueobject.PayoutStatus, error) {
	if !valueobject.CanCancel(status) {
		return status, entity.NewError("PAYOUT_CANCEL_REJECTED", "payout can be canceled only while PENDING")
	}
	return valueobject.PayoutCanceled, nil
}

// TransitionPayout enforces the payout state machine.
func TransitionPayout(from, to valueobject.PayoutStatus) (valueobject.PayoutStatus, error) {
	if !valueobject.CanTransition(from, to) {
		return from, entity.NewError("PAYOUT_TRANSITION_ILLEGAL", "illegal payout transition")
	}
	return to, nil
}

// IsOverdue reports whether actual settlement exceeded expected + grace.
func IsOverdue(expectedAt, actualAt time.Time, grace time.Duration) bool {
	return actualAt.After(expectedAt.Add(grace))
}

// ExpectedSettlementAt adds the rail lag to submission time.
func ExpectedSettlementAt(method valueobject.PayoutMethod, submittedAt time.Time) (time.Time, error) {
	lag, err := valueobject.ExpectedSettlementLag(method)
	if err != nil {
		return time.Time{}, entity.NewError("PAYOUT_METHOD_UNKNOWN", "payout method is unknown")
	}
	return submittedAt.Add(lag), nil
}
