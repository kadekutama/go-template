package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestReconciliationRunValidate(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseRun := entity.ReconciliationRun{
		RunID:         "run-123",
		TenantID:      "tenant-456",
		LedgerID:      "ledger-789",
		StartedAt:     baseTime,
		RuleVersion:   "v1",
		DecisionActor: "recon-service",
	}

	type testCase struct {
		name          string
		run           entity.ReconciliationRun
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid run",
			run:           baseRun,
			expectedError: nil,
		},
		{
			name: "missing run id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.RunID = ""
				return r
			}(),
			expectedError: entity.NewError("RUN_ID_REQUIRED", "reconciliation run id is required"),
		},
		{
			name: "whitespace run id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.RunID = "   "
				return r
			}(),
			expectedError: entity.NewError("RUN_ID_REQUIRED", "reconciliation run id is required"),
		},
		{
			name: "missing tenant id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.TenantID = ""
				return r
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "whitespace tenant id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.TenantID = "   "
				return r
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "missing ledger id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.LedgerID = ""
				return r
			}(),
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "whitespace ledger id",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.LedgerID = "   "
				return r
			}(),
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name: "missing rule version",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.RuleVersion = ""
				return r
			}(),
			expectedError: entity.NewError("RULE_VERSION_REQUIRED", "match rule version is required"),
		},
		{
			name: "whitespace rule version",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.RuleVersion = "   "
				return r
			}(),
			expectedError: entity.NewError("RULE_VERSION_REQUIRED", "match rule version is required"),
		},
		{
			name: "zero started at",
			run: func() entity.ReconciliationRun {
				r := baseRun
				r.StartedAt = time.Time{}
				return r
			}(),
			expectedError: entity.NewError("RUN_START_REQUIRED", "run start time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestExternalStatementLineValidate(t *testing.T) {
	t.Parallel()

	baseStart := time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)
	baseEnd := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	baseLine := entity.ExternalStatementLine{
		LineID:        "line-1",
		SourceAccount: "bank-acct-1",
		Reference:     "ref-001",
		AmountMinor:   10000,
		AssetCode:     "USD",
		EffectiveAt:   baseStart.Add(12 * time.Hour),
		Hash:          "hash-abc",
		ParserVersion: "v1",
		RawLineage:    "raw-file.csv:42",
		CoverageStart: baseStart,
		CoverageEnd:   baseEnd,
	}

	type testCase struct {
		name          string
		line          entity.ExternalStatementLine
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid statement line",
			line:          baseLine,
			expectedError: nil,
		},
		{
			name: "missing line id",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.LineID = ""
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line id is required"),
		},
		{
			name: "whitespace line id",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.LineID = "   "
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line id is required"),
		},
		{
			name: "missing source account",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.SourceAccount = ""
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line source account is required"),
		},
		{
			name: "whitespace source account",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.SourceAccount = "   "
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line source account is required"),
		},
		{
			name: "missing hash",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.Hash = ""
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line hash is required"),
		},
		{
			name: "whitespace hash",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.Hash = "   "
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line hash is required"),
		},
		{
			name: "missing parser version",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.ParserVersion = ""
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line parser version is required"),
		},
		{
			name: "whitespace parser version",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.ParserVersion = "   "
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line parser version is required"),
		},
		{
			name: "missing raw lineage",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.RawLineage = ""
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line raw lineage is required"),
		},
		{
			name: "whitespace raw lineage",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.RawLineage = "   "
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line raw lineage is required"),
		},
		{
			name: "zero coverage start",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.CoverageStart = time.Time{}
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line coverage interval is required"),
		},
		{
			name: "zero coverage end",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.CoverageEnd = time.Time{}
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line coverage interval is required"),
		},
		{
			name: "equal coverage start and end",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.CoverageEnd = l.CoverageStart
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "coverage start must precede end"),
		},
		{
			name: "coverage start after coverage end",
			line: func() entity.ExternalStatementLine {
				l := baseLine
				l.CoverageStart = baseEnd
				l.CoverageEnd = baseStart
				return l
			}(),
			expectedError: entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "coverage start must precede end"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.line.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
