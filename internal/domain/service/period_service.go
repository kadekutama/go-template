// Package service holds domain services: cross-aggregate rules that do not
// belong to a single aggregate root.
package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// SubLedgerKey partitions ledger facts per tenant, ledger, and asset without
// ever summing unlike assets.
type SubLedgerKey struct {
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	AssetCode valueobject.AssetCode
}

// Key builds a sub-ledger partition key.
func Key(tenant valueobject.TenantID, ledger valueobject.LedgerID, asset valueobject.AssetCode) SubLedgerKey {
	return SubLedgerKey{TenantID: tenant, LedgerID: ledger, AssetCode: asset}
}

// Matches reports whether the scope triple equals the key.
func (k SubLedgerKey) Matches(tenant valueobject.TenantID, ledger valueobject.LedgerID, asset valueobject.AssetCode) bool {
	return k.TenantID == tenant && k.LedgerID == ledger && k.AssetCode == asset
}

// BelongsToSubLedger reports whether an entry belongs to the partition: its
// account must sit in the key tenant+ledger and its asset must equal the key
// asset. Unknown accounts never match.
func BelongsToSubLedger(entry entity.Entry, accounts map[valueobject.AccountID]entity.AccountData, key SubLedgerKey) bool {
	acct, ok := accounts[entry.AccountID]
	if !ok {
		return false
	}
	return key.Matches(acct.TenantID, acct.LedgerID, entry.AssetCode) && acct.AssetCode == entry.AssetCode
}

// OpeningBalanceLine is one side of an opening-balance import.
type OpeningBalanceLine struct {
	AccountID   valueobject.AccountID
	Side        valueobject.Direction
	AmountMinor int64
	AssetCode   valueobject.AssetCode
}

// EvidenceRef records the source, evidence, and approval for an
// opening-balance import. The import never writes a balance column or bypasses
// the normal posting transaction.
type EvidenceRef struct {
	Source     string
	URI        string
	ApprovedBy string
}

// ValidateOpeningBalance checks that an opening-balance import may proceed:
// the period is open, lines are positive and balanced per asset, and
// source/evidence/approval are present. It constructs nothing.
func ValidateOpeningBalance(period entity.PeriodData, lines []OpeningBalanceLine, ev EvidenceRef) error {
	if period.Status != entity.PeriodOpen {
		return entity.NewError("PERIOD_CLOSED", "opening balances require an open period")
	}
	if len(lines) == 0 {
		return entity.NewError("OPENING_LINES_REQUIRED", "opening balance requires at least one line")
	}
	if ev.Source == "" || ev.URI == "" || ev.ApprovedBy == "" {
		return entity.NewError("EVIDENCE_REQUIRED", "opening balance requires source, evidence URI, and approver")
	}
	totals, err := accumulateOpeningTotals(lines)
	if err != nil {
		return err
	}
	return verifyOpeningTotals(totals)
}

func validateOpeningLine(l OpeningBalanceLine) error {
	if l.AccountID.String() == "" {
		return entity.NewError("OPENING_ACCOUNT_REQUIRED", "opening line account is required")
	}
	if l.Side != valueobject.DirectionDebit && l.Side != valueobject.DirectionCredit {
		return entity.NewError("OPENING_SIDE_INVALID", "opening line side must be DEBIT or CREDIT")
	}
	if l.AmountMinor <= 0 {
		return entity.NewError("INVALID_ENTRY_AMOUNT", "opening line amount must be positive")
	}
	if l.AssetCode == "" {
		return entity.NewError("OPENING_ASSET_REQUIRED", "opening line asset is required")
	}
	return nil
}

func accumulateOpeningTotals(lines []OpeningBalanceLine) (map[valueobject.AssetCode][2]int64, error) {
	totals := make(map[valueobject.AssetCode][2]int64)
	for _, l := range lines {
		if err := validateOpeningLine(l); err != nil {
			return nil, err
		}
		t := totals[l.AssetCode]
		if l.Side == valueobject.DirectionDebit {
			t[0] += l.AmountMinor
		} else {
			t[1] += l.AmountMinor
		}
		totals[l.AssetCode] = t
	}
	return totals, nil
}

func verifyOpeningTotals(totals map[valueobject.AssetCode][2]int64) error {
	for asset, t := range totals {
		if t[0] != t[1] {
			return entity.Errorf("UNBALANCED_TRANSACTION", "opening lines unbalanced for asset %s", asset)
		}
	}
	return nil
}
