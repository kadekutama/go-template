package valueobject

import (
	"fmt"
)

// BreakType is the reconciliation break taxonomy.
type BreakType string

// Reconciliation break types.
const (
	BreakMissingInLedger BreakType = "MISSING_IN_LEDGER"
	BreakMissingInBank   BreakType = "MISSING_IN_BANK"
	BreakAmountMismatch  BreakType = "AMOUNT_MISMATCH"
	BreakDateMismatch    BreakType = "DATE_MISMATCH"
	BreakDuplicate       BreakType = "DUPLICATE"
)

// ParseBreakType validates a break type.
func ParseBreakType(s string) (BreakType, error) {
	switch BreakType(s) {
	case BreakMissingInLedger, BreakMissingInBank, BreakAmountMismatch, BreakDateMismatch, BreakDuplicate:
		return BreakType(s), nil
	default:
		return "", fmt.Errorf("reconciliation: invalid break type %q", s)
	}
}
