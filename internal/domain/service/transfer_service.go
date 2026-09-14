package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TransferRequest is an immediate (or execution-time) transfer command. Funds
// come from a caller-supplied strong-read available snapshot; the domain
// never reads stores.
type TransferRequest struct {
	TenantID      valueobject.TenantID
	LedgerID      valueobject.LedgerID
	Source        valueobject.AccountID
	Dest          valueobject.AccountID
	AssetCode     valueobject.AssetCode
	AmountMinor   int64
	FXRatePresent bool
}

// TransferLines is the template-based posting shape: debit the source
// liability, credit the destination liability.
type TransferLines struct {
	DebitAccount  valueobject.AccountID
	CreditAccount valueobject.AccountID
	AmountMinor   int64
	AssetCode     valueobject.AssetCode
}

// ValidateImmediate checks an immediate transfer and builds its posting lines.
func ValidateImmediate(req TransferRequest, accounts map[valueobject.AccountID]entity.AccountData, sourceAvailableMinor int64) (TransferLines, error) {
	if req.AmountMinor <= 0 {
		return TransferLines{}, entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive")
	}
	if req.Source == req.Dest {
		return TransferLines{}, entity.NewError("SELF_TRANSFER_REJECTED", "transfer source and destination must differ")
	}
	src, ok := accounts[req.Source]
	if !ok {
		return TransferLines{}, entity.NewError("SOURCE_ACCOUNT_NOT_FOUND", "source account is unknown")
	}
	dst, ok := accounts[req.Dest]
	if !ok {
		return TransferLines{}, entity.NewError("DEST_ACCOUNT_NOT_FOUND", "destination account is unknown")
	}
	if err := checkTransferScope(req, src, dst); err != nil {
		return TransferLines{}, err
	}
	if err := checkTransferAccountActive(src); err != nil {
		return TransferLines{}, err
	}
	if err := checkTransferAccountActive(dst); err != nil {
		return TransferLines{}, err
	}
	if err := checkTransferAssets(req, src, dst); err != nil {
		return TransferLines{}, err
	}
	if sourceAvailableMinor < req.AmountMinor {
		return TransferLines{}, entity.NewError("INSUFFICIENT_FUNDS", "source available balance below transfer amount")
	}
	return TransferLines{
		DebitAccount:  req.Source,
		CreditAccount: req.Dest,
		AmountMinor:   req.AmountMinor,
		AssetCode:     req.AssetCode,
	}, nil
}

func checkTransferScope(req TransferRequest, src, dst entity.AccountData) error {
	if src.TenantID != req.TenantID || dst.TenantID != req.TenantID {
		return entity.NewError("TENANT_MISMATCH", "transfer accounts must share the request tenant")
	}
	if src.LedgerID != req.LedgerID || dst.LedgerID != req.LedgerID {
		return entity.NewError("LEDGER_MISMATCH", "transfer accounts must share the request ledger")
	}
	return nil
}

func checkTransferAccountActive(a entity.AccountData) error {
	switch a.Status {
	case valueobject.StatusActive:
		return nil
	case valueobject.StatusFrozen:
		return entity.NewError("ACCOUNT_FROZEN", "account is frozen")
	case valueobject.StatusClosed:
		return entity.NewError("ACCOUNT_CLOSED", "account is closed")
	default:
		return entity.NewError("ACCOUNT_STATUS_INVALID", "account status is invalid")
	}
}

func checkTransferAssets(req TransferRequest, src, dst entity.AccountData) error {
	if src.AssetCode == req.AssetCode && dst.AssetCode == req.AssetCode {
		return nil
	}
	// A leg in another asset is a cross-currency transfer: allowed only with
	// an FX rate whose conversion lots are built by the FX service (E03-T05).
	if !req.FXRatePresent {
		return entity.NewError("CURRENCY_MISMATCH", "transfer asset must match account assets")
	}
	if req.AssetCode != src.AssetCode && req.AssetCode != dst.AssetCode {
		return entity.NewError("CURRENCY_MISMATCH", "transfer asset must match at least one leg")
	}
	return nil
}
