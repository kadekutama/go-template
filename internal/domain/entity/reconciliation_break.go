package entity

import (
	"strings"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ReconciliationBreak is the persistence record for one mismatch.
type ReconciliationBreak struct {
	BreakID       string
	RunID         string
	TenantID      string
	Type          valueobject.BreakType
	LedgerRef     string
	ExternalRef   string
	ExpectedMinor int64
	ActualMinor   int64
	Evidence      map[string]string
	RuleVersion   string
}

// Validate checks break identity and per-type evidence.
func (b ReconciliationBreak) Validate() error {
	if strings.TrimSpace(b.BreakID) == "" {
		return NewError("BREAK_ID_REQUIRED", "break id is required")
	}
	if strings.TrimSpace(b.RunID) == "" {
		return NewError("RUN_ID_REQUIRED", "run id is required")
	}
	if strings.TrimSpace(b.TenantID) == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if _, err := valueobject.ParseBreakType(string(b.Type)); err != nil {
		return NewError("BREAK_TYPE_INVALID", "break type is invalid")
	}
	if strings.TrimSpace(b.RuleVersion) == "" {
		return NewError("RULE_VERSION_REQUIRED", "rule version is required")
	}
	return validateBreakEvidence(b)
}

func validateBreakEvidence(b ReconciliationBreak) error {
	switch b.Type {
	case valueobject.BreakMissingInLedger:
		return requireExternalRef(b)
	case valueobject.BreakMissingInBank:
		return requireLedgerRef(b)
	case valueobject.BreakAmountMismatch, valueobject.BreakDateMismatch:
		return requireBothRefs(b)
	case valueobject.BreakDuplicate:
		return requireDuplicateEvidence(b)
	default:
		return NewError("BREAK_TYPE_INVALID", "break type is invalid")
	}
}

func requireExternalRef(b ReconciliationBreak) error {
	if strings.TrimSpace(b.ExternalRef) == "" {
		return NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-ledger requires the external reference")
	}
	return nil
}

func requireLedgerRef(b ReconciliationBreak) error {
	if strings.TrimSpace(b.LedgerRef) == "" {
		return NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-bank requires the ledger reference")
	}
	return nil
}

func requireBothRefs(b ReconciliationBreak) error {
	if strings.TrimSpace(b.LedgerRef) == "" || strings.TrimSpace(b.ExternalRef) == "" {
		return NewError("BREAK_EVIDENCE_REQUIRED", "mismatch requires both references")
	}
	return nil
}

func requireDuplicateEvidence(b ReconciliationBreak) error {
	if strings.TrimSpace(b.ExternalRef) == "" {
		return NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires the external reference")
	}
	if b.Evidence == nil || strings.TrimSpace(b.Evidence["duplicate_of"]) == "" {
		return NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires duplicate_of evidence")
	}
	return nil
}
