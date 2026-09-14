package entity

import (
	"strings"
	"time"
)

// ReconciliationRun is the persistence record for one daily run.
type ReconciliationRun struct {
	RunID         string
	TenantID      string
	LedgerID      string
	StartedAt     time.Time
	RuleVersion   string
	DecisionActor string
}

// Validate checks run identity.
func (r ReconciliationRun) Validate() error {
	if strings.TrimSpace(r.RunID) == "" {
		return NewError("RUN_ID_REQUIRED", "reconciliation run id is required")
	}
	if strings.TrimSpace(r.TenantID) == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(r.LedgerID) == "" {
		return NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if strings.TrimSpace(r.RuleVersion) == "" {
		return NewError("RULE_VERSION_REQUIRED", "match rule version is required")
	}
	if r.StartedAt.IsZero() {
		return NewError("RUN_START_REQUIRED", "run start time is required")
	}
	return nil
}

// ExternalStatementLine is an immutable external source snapshot line.
type ExternalStatementLine struct {
	LineID        string
	SourceAccount string
	Reference     string
	AmountMinor   int64
	AssetCode     string
	EffectiveAt   time.Time
	Hash          string
	ParserVersion string
	RawLineage    string
	CoverageStart time.Time
	CoverageEnd   time.Time
}

// Validate checks snapshot identity.
func (l ExternalStatementLine) Validate() error {
	if strings.TrimSpace(l.LineID) == "" {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line id is required")
	}
	if strings.TrimSpace(l.SourceAccount) == "" {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line source account is required")
	}
	if strings.TrimSpace(l.Hash) == "" {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line hash is required")
	}
	if strings.TrimSpace(l.ParserVersion) == "" {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line parser version is required")
	}
	if strings.TrimSpace(l.RawLineage) == "" {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line raw lineage is required")
	}
	if l.CoverageStart.IsZero() || l.CoverageEnd.IsZero() {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line coverage interval is required")
	}
	if !l.CoverageStart.Before(l.CoverageEnd) {
		return NewError("SNAPSHOT_IDENTITY_REQUIRED", "coverage start must precede end")
	}
	return nil
}
