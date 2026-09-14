package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TopupRequest funds platform/merchant balance from a verified external bank
// instrument: the reverse of a payout. The instrument is a tokenized external
// reference, never a ledger account.
type TopupRequest struct {
	ID                 string
	TenantID           valueobject.TenantID
	LedgerID           valueobject.LedgerID
	CreditAccount      valueobject.AccountID
	AmountMinor        int64
	AssetCode          valueobject.AssetCode
	InstrumentID       string
	InstrumentVerified bool
	IdempotencyKey     string
}

// TopupFunding is the funding journal shape (reverse settlement tracking):
// DR Bank Cash / CR Merchant Payable (available).
type TopupFunding struct {
	BankCash        valueobject.AccountID
	MerchantAccount valueobject.AccountID
	AmountMinor     int64
	AssetCode       valueobject.AssetCode
	TraceID         string
}

// ValidateTopup gates on verified instrument, positive amount, and key.
func ValidateTopup(req TopupRequest) (TopupFunding, error) {
	if !req.InstrumentVerified || req.InstrumentID == "" {
		return TopupFunding{}, entity.NewError("ACCOUNT_UNVERIFIED", "top-up requires a verified external bank instrument")
	}
	if req.AmountMinor <= 0 {
		return TopupFunding{}, entity.NewError("INVALID_TOPUP_AMOUNT", "top-up amount must be positive")
	}
	if req.IdempotencyKey == "" {
		return TopupFunding{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "top-up requires an idempotency key")
	}
	if req.CreditAccount.String() == "" {
		return TopupFunding{}, entity.NewError("TOPUP_ACCOUNT_REQUIRED", "top-up requires a credit account")
	}
	if req.AssetCode == "" {
		return TopupFunding{}, entity.NewError("TOPUP_ASSET_REQUIRED", "top-up requires an asset code")
	}
	return TopupFunding{
		MerchantAccount: req.CreditAccount,
		AmountMinor:     req.AmountMinor,
		AssetCode:       req.AssetCode,
	}, nil
}

// SettleTopup resolves PENDING to SUCCEEDED (with funding lines) or FAILED
// (moving nothing).
func SettleTopup(status valueobject.TopupStatus, funding TopupFunding, bankCash valueobject.AccountID, traceID string, succeeded bool) (valueobject.TopupStatus, *TopupFunding, error) {
	target := valueobject.TopupFailed
	if succeeded {
		target = valueobject.TopupSucceeded
	}
	if !valueobject.CanTransitionTopup(status, target) {
		return status, nil, entity.NewError("TOPUP_STATE_INVALID", "top-up can settle only while PENDING")
	}
	if bankCash.String() == "" {
		return valueobject.TopupFailed, nil, entity.NewError("TOPUP_ACCOUNT_REQUIRED", "settlement requires a bank cash account")
	}
	if !succeeded {
		return valueobject.TopupFailed, nil, nil
	}
	if funding.AmountMinor <= 0 {
		return valueobject.TopupFailed, nil, entity.NewError("INVALID_TOPUP_AMOUNT", "top-up amount must be positive")
	}
	if bankCash == funding.MerchantAccount {
		return valueobject.TopupFailed, nil, entity.NewError("TOPUP_ACCOUNT_INVALID", "settlement accounts must be distinct")
	}
	funding.BankCash = bankCash
	funding.TraceID = traceID
	return valueobject.TopupSucceeded, &funding, nil
}

// CancelTopup allows cancel while PENDING only.
func CancelTopup(status valueobject.TopupStatus) (valueobject.TopupStatus, error) {
	if !valueobject.CanTransitionTopup(status, valueobject.TopupCanceled) {
		return status, entity.NewError("TOPUP_CANCEL_REJECTED", "top-up can be canceled only while PENDING")
	}
	return valueobject.TopupCanceled, nil
}
