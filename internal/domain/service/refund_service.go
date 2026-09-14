package service

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// DefaultRefundWindowDays is the configurable default refund window.
const DefaultRefundWindowDays = 90

// FeeRefundPolicy carries the explicit processor/platform fee-refund outcome.
// Proportions are never assumed.
type FeeRefundPolicy struct {
	ProcessorFeeRefundMinor int64
	PlatformFeeRefundMinor  int64
}

// RefundRequest is a refund validation command. Times are explicit params.
// The three posting accounts mirror ledger-core §6.4: acceptance debits the
// merchant payable and credits refunds payable; settlement debits refunds
// payable and credits bank cash.
type RefundRequest struct {
	OriginalPostingID   valueobject.PostingID
	OriginalAmountMinor int64
	PriorRefundedMinor  int64
	AmountMinor         int64
	OriginalAt          time.Time
	Now                 time.Time
	WindowDays          int
	MerchantPayable     valueobject.AccountID
	RefundsPayable      valueobject.AccountID
	CashAccount         valueobject.AccountID
	AssetCode           valueobject.AssetCode
	FeePolicy           FeeRefundPolicy
}

// RefundLines are the acceptance + settlement posting shapes linked to the
// original posting. Original entries stay immutable.
type RefundLines struct {
	OriginalPostingID valueobject.PostingID
	AcceptanceDebit   valueobject.AccountID
	AcceptanceCredit  valueobject.AccountID
	SettlementDebit   valueobject.AccountID
	SettlementCredit  valueobject.AccountID
	AmountMinor       int64
	AssetCode         valueobject.AssetCode
}

// ValidateRefund enforces original existence, amount caps, window, and
// account scope/status/asset, then builds linked lines.
func ValidateRefund(req RefundRequest, originalExists bool, accounts map[valueobject.AccountID]entity.AccountData) (RefundLines, error) {
	if err := validateRefundAmounts(req, originalExists); err != nil {
		return RefundLines{}, err
	}
	if err := validateRefundWindow(req); err != nil {
		return RefundLines{}, err
	}
	if err := validateRefundAccounts(req, accounts); err != nil {
		return RefundLines{}, err
	}
	if err := validateRefundFeePolicy(req.FeePolicy); err != nil {
		return RefundLines{}, err
	}
	return RefundLines{
		OriginalPostingID: req.OriginalPostingID,
		AcceptanceDebit:   req.MerchantPayable,
		AcceptanceCredit:  req.RefundsPayable,
		SettlementDebit:   req.RefundsPayable,
		SettlementCredit:  req.CashAccount,
		AmountMinor:       req.AmountMinor,
		AssetCode:         req.AssetCode,
	}, nil
}

func validateRefundAmounts(req RefundRequest, originalExists bool) error {
	if !originalExists || req.OriginalPostingID.String() == "" {
		return entity.NewError("ORIGINAL_NOT_FOUND", "original payment does not exist")
	}
	if req.AmountMinor <= 0 {
		return entity.NewError("INVALID_REFUND_AMOUNT", "refund amount must be positive")
	}
	if req.OriginalAmountMinor <= 0 {
		return entity.NewError("ORIGINAL_AMOUNT_INVALID", "original amount must be positive")
	}
	if req.PriorRefundedMinor < 0 || req.PriorRefundedMinor > req.OriginalAmountMinor {
		return entity.NewError("REFUND_EXCEEDS_ORIGINAL", "prior refunds exceed original amount")
	}
	if req.AmountMinor > req.OriginalAmountMinor-req.PriorRefundedMinor {
		return entity.NewError("REFUND_EXCEEDS_ORIGINAL", "refund exceeds remaining refundable amount")
	}
	return nil
}

func validateRefundWindow(req RefundRequest) error {
	window := req.WindowDays
	if window <= 0 {
		window = DefaultRefundWindowDays
	}
	if req.Now.Sub(req.OriginalAt) > time.Duration(window)*24*time.Hour {
		return entity.NewError("REFUND_WINDOW_EXPIRED", "refund window has expired")
	}
	return nil
}

func validateRefundAccounts(req RefundRequest, accounts map[valueobject.AccountID]entity.AccountData) error {
	parties := []valueobject.AccountID{req.MerchantPayable, req.RefundsPayable, req.CashAccount}
	for _, id := range parties {
		if id.String() == "" {
			return entity.NewError("REFUND_ACCOUNT_REQUIRED", "refund requires merchant, refunds-payable, and cash accounts")
		}
	}
	if req.MerchantPayable == req.RefundsPayable || req.RefundsPayable == req.CashAccount || req.MerchantPayable == req.CashAccount {
		return entity.NewError("REFUND_ACCOUNT_INVALID", "refund posting accounts must be distinct")
	}
	for _, id := range parties {
		acct, ok := accounts[id]
		if !ok {
			return entity.NewError("REFUND_ACCOUNT_NOT_FOUND", "refund account is unknown")
		}
		switch acct.Status {
		case valueobject.StatusActive:
		case valueobject.StatusFrozen:
			return entity.NewError("ACCOUNT_FROZEN", "refund account is frozen")
		case valueobject.StatusClosed:
			return entity.NewError("ACCOUNT_CLOSED", "refund account is closed")
		default:
			return entity.NewError("ACCOUNT_STATUS_INVALID", "refund account status is invalid")
		}
		if acct.AssetCode != req.AssetCode {
			return entity.NewError("CURRENCY_MISMATCH", "refund account asset must match the refund asset")
		}
	}
	return nil
}

func validateRefundFeePolicy(policy FeeRefundPolicy) error {
	if policy.ProcessorFeeRefundMinor < 0 || policy.PlatformFeeRefundMinor < 0 {
		return entity.NewError("FEE_POLICY_INVALID", "fee refunds must be explicit non-negative amounts")
	}
	return nil
}
