package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TransferTemplate is a stored parameter set that pre-fills transfers (P2).
// Applying a template never weakens validation.
type TransferTemplate struct {
	ID          string
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	Source      valueobject.AccountID
	Dest        valueobject.AccountID
	AssetCode   valueobject.AssetCode
	AmountMinor int64
}

// ValidateTemplate checks the stored parameter set.
func ValidateTemplate(t TransferTemplate) error {
	if t.ID == "" {
		return entity.NewError("TEMPLATE_ID_REQUIRED", "template id is required")
	}
	if t.TenantID.String() == "" || t.LedgerID.String() == "" {
		return entity.NewError("TENANT_MISMATCH", "template requires tenant and ledger scope")
	}
	if t.Source.String() == "" || t.Dest.String() == "" {
		return entity.NewError("TEMPLATE_ACCOUNT_REQUIRED", "template requires source and destination accounts")
	}
	if t.AssetCode == "" {
		return entity.NewError("TEMPLATE_ASSET_REQUIRED", "template requires an asset code")
	}
	if t.AmountMinor <= 0 {
		return entity.NewError("INVALID_TRANSFER_AMOUNT", "template amount must be positive")
	}
	return nil
}

// ApplyTemplate pre-fills a transfer request from the template. Amount and
// FX overrides are explicit; scope always comes from the template.
func ApplyTemplate(t TransferTemplate, amountMinor int64, fxRatePresent bool) (TransferRequest, error) {
	if err := ValidateTemplate(t); err != nil {
		return TransferRequest{}, err
	}
	amount := t.AmountMinor
	if amountMinor > 0 {
		amount = amountMinor
	}
	if amount <= 0 {
		return TransferRequest{}, entity.NewError("INVALID_TRANSFER_AMOUNT", "transfer amount must be positive")
	}
	return TransferRequest{
		TenantID:      t.TenantID,
		LedgerID:      t.LedgerID,
		Source:        t.Source,
		Dest:          t.Dest,
		AssetCode:     t.AssetCode,
		AmountMinor:   amount,
		FXRatePresent: fxRatePresent,
	}, nil
}
