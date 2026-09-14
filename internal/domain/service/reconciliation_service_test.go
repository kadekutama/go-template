package service_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestMatchStatements(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 9, 14, 2, 0, 0, 0, time.UTC)
	baseCfg := service.MatchConfig{
		RuleVersion:  "v1",
		TimingWindow: 24 * time.Hour,
		Tolerances: []service.TolerancePolicy{
			{Rail: "ACH", AssetCode: "USD", ToleranceMinor: 0},
		},
		Actor:    "recon-engine",
		Rail:     "ACH",
		RunID:    "run-1",
		TenantID: "t-1",
	}

	baseLedger := []service.LedgerFact{
		{
			PostingID:   "p-exact",
			Reference:   "ref-exact",
			AmountMinor: 10000,
			AssetCode:   "USD",
			EffectiveAt: baseTime,
		},
		{
			PostingID:   "p-split",
			Reference:   "ref-split",
			AmountMinor: 10000,
			AssetCode:   "USD",
			EffectiveAt: baseTime,
		},
		{
			PostingID:   "p-only",
			Reference:   "ref-only-ledger",
			AmountMinor: 5000,
			AssetCode:   "USD",
			EffectiveAt: baseTime,
		},
	}

	baseExternal := []entity.ExternalStatementLine{
		{
			LineID:        "e-exact",
			SourceAccount: "bank-1",
			Reference:     "ref-exact",
			AmountMinor:   10000,
			AssetCode:     "USD",
			EffectiveAt:   baseTime,
			Hash:          "h-exact",
			ParserVersion: "csv-v1",
			RawLineage:    "sftp/bank-1/2026-09-14.csv:1",
			CoverageStart: baseTime.Add(-24 * time.Hour),
			CoverageEnd:   baseTime.Add(24 * time.Hour),
		},
		{
			LineID:        "e-split-1",
			SourceAccount: "bank-1",
			Reference:     "ref-split",
			AmountMinor:   6000,
			AssetCode:     "USD",
			EffectiveAt:   baseTime,
			Hash:          "h-split-1",
			ParserVersion: "csv-v1",
			RawLineage:    "sftp/bank-1/2026-09-14.csv:2",
			CoverageStart: baseTime.Add(-24 * time.Hour),
			CoverageEnd:   baseTime.Add(24 * time.Hour),
		},
		{
			LineID:        "e-split-2",
			SourceAccount: "bank-1",
			Reference:     "ref-split",
			AmountMinor:   4000,
			AssetCode:     "USD",
			EffectiveAt:   baseTime,
			Hash:          "h-split-2",
			ParserVersion: "csv-v1",
			RawLineage:    "sftp/bank-1/2026-09-14.csv:3",
			CoverageStart: baseTime.Add(-24 * time.Hour),
			CoverageEnd:   baseTime.Add(24 * time.Hour),
		},
	}

	type testCase struct {
		name            string
		ledger          []service.LedgerFact
		external        []entity.ExternalStatementLine
		cfg             service.MatchConfig
		expectedMatched []service.MatchGroup
		expectedBreaks  []entity.ReconciliationBreak
		expectedError   error
	}

	testCases := []testCase{
		{
			name:     "golden exact plus split plus missing",
			ledger:   baseLedger,
			external: baseExternal,
			cfg:      baseCfg,
			expectedMatched: []service.MatchGroup{
				{
					GroupID:       "grp:run-1:ref-exact:0",
					RuleVersion:   "v1",
					Confidence:    "HIGH",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-exact"},
					ExternalRefs:  []string{"e-exact"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
				{
					GroupID:       "grp:run-1:ref-split:0",
					RuleVersion:   "v1",
					Confidence:    "MEDIUM",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-split"},
					ExternalRefs:  []string{"e-split-2", "e-split-1"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
			},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:       "brk:run-1:MISSING_IN_BANK:ref-only-ledger:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakMissingInBank,
					LedgerRef:     "p-only",
					ExpectedMinor: 5000,
					Evidence:      map[string]string{"reference": "ref-only-ledger"},
					RuleVersion:   "v1",
				},
			},
			expectedError: nil,
		},
		{
			name:   "amount mismatch beyond tolerance",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				out[0].AmountMinor = 9999
				out[0].Hash = "h-exact-diff"
				return out
			}(),
			cfg: baseCfg,
			expectedMatched: []service.MatchGroup{
				{
					GroupID:       "grp:run-1:ref-split:0",
					RuleVersion:   "v1",
					Confidence:    "MEDIUM",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-split"},
					ExternalRefs:  []string{"e-split-2", "e-split-1"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
			},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:       "brk:run-1:AMOUNT_MISMATCH:ref-exact:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakAmountMismatch,
					LedgerRef:     "p-exact",
					ExternalRef:   "e-exact",
					ExpectedMinor: 10000,
					ActualMinor:   9999,
					Evidence:      map[string]string{"reference": "ref-exact", "tolerance_minor": "0"},
					RuleVersion:   "v1",
				},
				{
					BreakID:       "brk:run-1:MISSING_IN_BANK:ref-only-ledger:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakMissingInBank,
					LedgerRef:     "p-only",
					ExpectedMinor: 5000,
					Evidence:      map[string]string{"reference": "ref-only-ledger"},
					RuleVersion:   "v1",
				},
			},
			expectedError: nil,
		},
		{
			name:   "date mismatch beyond window",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				out[0].EffectiveAt = baseTime.Add(48 * time.Hour)
				return out
			}(),
			cfg: baseCfg,
			expectedMatched: []service.MatchGroup{
				{
					GroupID:       "grp:run-1:ref-split:0",
					RuleVersion:   "v1",
					Confidence:    "MEDIUM",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-split"},
					ExternalRefs:  []string{"e-split-2", "e-split-1"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
			},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:       "brk:run-1:DATE_MISMATCH:ref-exact:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakDateMismatch,
					LedgerRef:     "p-exact",
					ExternalRef:   "e-exact",
					ExpectedMinor: 10000,
					ActualMinor:   10000,
					Evidence:      map[string]string{"reference": "ref-exact", "timing_window": "24h0m0s"},
					RuleVersion:   "v1",
				},
				{
					BreakID:       "brk:run-1:MISSING_IN_BANK:ref-only-ledger:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakMissingInBank,
					LedgerRef:     "p-only",
					ExpectedMinor: 5000,
					Evidence:      map[string]string{"reference": "ref-only-ledger"},
					RuleVersion:   "v1",
				},
			},
			expectedError: nil,
		},
		{
			name:   "duplicate external reference same hash",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				dup := out[0]
				dup.LineID = "e-exact-dup"
				out = append(out, dup)
				return out
			}(),
			cfg: baseCfg,
			expectedMatched: []service.MatchGroup{
				{
					GroupID:       "grp:run-1:ref-exact:0",
					RuleVersion:   "v1",
					Confidence:    "HIGH",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-exact"},
					ExternalRefs:  []string{"e-exact"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
				{
					GroupID:       "grp:run-1:ref-split:0",
					RuleVersion:   "v1",
					Confidence:    "MEDIUM",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-split"},
					ExternalRefs:  []string{"e-split-2", "e-split-1"},
					LedgerSum:     10000,
					ExternalSum:   10000,
					DecisionActor: "recon-engine",
				},
			},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:     "brk:run-1:DUPLICATE:ref-exact:0",
					RunID:       "run-1",
					TenantID:    "t-1",
					Type:        valueobject.BreakDuplicate,
					ExternalRef: "e-exact-dup",
					Evidence:    map[string]string{"duplicate_of": "ref-exact", "hash": "h-exact"},
					RuleVersion: "v1",
				},
				{
					BreakID:       "brk:run-1:MISSING_IN_BANK:ref-only-ledger:0",
					RunID:         "run-1",
					TenantID:      "t-1",
					Type:          valueobject.BreakMissingInBank,
					LedgerRef:     "p-only",
					ExpectedMinor: 5000,
					Evidence:      map[string]string{"reference": "ref-only-ledger"},
					RuleVersion:   "v1",
				},
			},
			expectedError: nil,
		},
		{
			name:   "orphan external yields missing in ledger",
			ledger: nil,
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-orphan",
					SourceAccount: "bank-1",
					Reference:     "ref-orphan",
					AmountMinor:   7500,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-orphan",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:9",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.RunID = "run-missing"
				return c
			}(),
			expectedMatched: []service.MatchGroup{},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:     "brk:run-missing:MISSING_IN_LEDGER:ref-orphan:0",
					RunID:       "run-missing",
					TenantID:    "t-1",
					Type:        valueobject.BreakMissingInLedger,
					ExternalRef: "e-orphan",
					ActualMinor: 7500,
					Evidence:    map[string]string{"reference": "ref-orphan"},
					RuleVersion: "v1",
				},
			},
			expectedError: nil,
		},
		{
			name:            "empty inputs match nothing",
			ledger:          nil,
			external:        nil,
			cfg:             baseCfg,
			expectedMatched: []service.MatchGroup{},
			expectedBreaks:  []entity.ReconciliationBreak{},
			expectedError:   nil,
		},
		{
			name: "cross-asset reference is amount mismatch",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-1",
					Reference:   "ref-fx",
					AmountMinor: 10000,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-fx",
					AmountMinor:   10000,
					AssetCode:     "EUR",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.RunID = "run-fx"
				c.Tolerances = []service.TolerancePolicy{
					{Rail: "ACH", AssetCode: "USD", ToleranceMinor: 0},
					{Rail: "ACH", AssetCode: "EUR", ToleranceMinor: 0},
				}
				return c
			}(),
			expectedMatched: []service.MatchGroup{},
			expectedBreaks: []entity.ReconciliationBreak{
				{
					BreakID:       "brk:run-fx:AMOUNT_MISMATCH:ref-fx:0",
					RunID:         "run-fx",
					TenantID:      "t-1",
					Type:          valueobject.BreakAmountMismatch,
					LedgerRef:     "p-1",
					ExternalRef:   "e-1",
					ExpectedMinor: 10000,
					ActualMinor:   10000,
					Evidence:      map[string]string{"reference": "ref-fx", "tolerance_minor": "0"},
					RuleVersion:   "v1",
				},
			},
			expectedError: nil,
		},
		{
			name: "tied sort keys match deterministically",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-b",
					Reference:   "ref-same",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
				{
					PostingID:   "p-a",
					Reference:   "ref-same",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
				{
					PostingID:   "p-c",
					Reference:   "ref-same",
					AmountMinor: 50,
					AssetCode:   "USD",
					EffectiveAt: baseTime.Add(time.Hour),
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-b",
					SourceAccount: "bank-1",
					Reference:     "ref-same",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-b",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:2",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(48 * time.Hour),
				},
				{
					LineID:        "e-a",
					SourceAccount: "bank-1",
					Reference:     "ref-same",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-a",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(48 * time.Hour),
				},
				{
					LineID:        "e-c",
					SourceAccount: "bank-1",
					Reference:     "ref-same",
					AmountMinor:   50,
					AssetCode:     "USD",
					EffectiveAt:   baseTime.Add(2 * time.Hour),
					Hash:          "h-c",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:3",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(48 * time.Hour),
				},
			},
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.RunID = "run-tie"
				c.TimingWindow = 48 * time.Hour
				return c
			}(),
			expectedMatched: []service.MatchGroup{
				{
					GroupID:       "grp:run-tie:ref-same:0",
					RuleVersion:   "v1",
					Confidence:    "MEDIUM",
					Decision:      valueobject.MatchMatched,
					LedgerRefs:    []string{"p-c", "p-a", "p-b"},
					ExternalRefs:  []string{"e-c", "e-a", "e-b"},
					LedgerSum:     250,
					ExternalSum:   250,
					DecisionActor: "recon-engine",
				},
			},
			expectedBreaks: []entity.ReconciliationBreak{},
			expectedError:  nil,
		},
		{
			name:   "missing tolerance policy",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Tolerances = nil
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance policy is required for rail and asset"),
		},
		{
			name:   "duplicate tolerance conflict",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Tolerances = []service.TolerancePolicy{
					{Rail: "ACH", AssetCode: "USD", ToleranceMinor: 0},
					{Rail: "ACH", AssetCode: "USD", ToleranceMinor: 5},
				}
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TOLERANCE_POLICY_CONFLICT", "duplicate tolerance policy for rail and asset"),
		},
		{
			name:   "negative tolerance rejected",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Tolerances = []service.TolerancePolicy{
					{Rail: "ACH", AssetCode: "USD", ToleranceMinor: -1},
				}
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance must be non-negative"),
		},
		{
			name:   "empty tolerance rail rejected",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Tolerances = []service.TolerancePolicy{
					{Rail: "", AssetCode: "USD", ToleranceMinor: 0},
				}
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TOLERANCE_POLICY_REQUIRED", "tolerance requires rail and asset"),
		},
		{
			name:   "missing snapshot identity",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				out[0].Hash = ""
				return out
			}(),
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("SNAPSHOT_IDENTITY_REQUIRED", "external line hash is required"),
		},
		{
			name: "empty ledger reference rejected",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-1",
					Reference:   "",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("LEDGER_REFERENCE_REQUIRED", "ledger fact reference is required"),
		},
		{
			name: "whitespace posting id rejected",
			ledger: []service.LedgerFact{
				{
					PostingID:   "   ",
					Reference:   "ref-1",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("POSTING_ID_REQUIRED", "ledger fact posting id is required"),
		},
		{
			name: "zero ledger amount rejected",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-1",
					Reference:   "ref-1",
					AmountMinor: 0,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("INVALID_ENTRY_AMOUNT", "ledger fact amount must be positive"),
		},
		{
			name: "overflow ledger sums error",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-1",
					Reference:   "ref-1",
					AmountMinor: 9223372036854775800,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
				{
					PostingID:   "p-2",
					Reference:   "ref-1",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("AMOUNT_OVERFLOW", "ledger sum overflowed int64"),
		},
		{
			name: "overflow external sums error",
			ledger: []service.LedgerFact{
				{
					PostingID:   "p-1",
					Reference:   "ref-1",
					AmountMinor: 100,
					AssetCode:   "USD",
					EffectiveAt: baseTime,
				},
			},
			external: []entity.ExternalStatementLine{
				{
					LineID:        "e-1",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   9223372036854775800,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-1",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:1",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
				{
					LineID:        "e-2",
					SourceAccount: "bank-1",
					Reference:     "ref-1",
					AmountMinor:   100,
					AssetCode:     "USD",
					EffectiveAt:   baseTime,
					Hash:          "h-2",
					ParserVersion: "csv-v1",
					RawLineage:    "sftp/x:2",
					CoverageStart: baseTime.Add(-24 * time.Hour),
					CoverageEnd:   baseTime.Add(24 * time.Hour),
				},
			},
			cfg:             baseCfg,
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("AMOUNT_OVERFLOW", "external sum overflowed int64"),
		},
		{
			name:   "missing rule version",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.RuleVersion = ""
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("RULE_VERSION_REQUIRED", "match rule version is required"),
		},
		{
			name:   "missing rail",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Rail = ""
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TOLERANCE_POLICY_REQUIRED", "match rail is required for tolerance lookup"),
		},
		{
			name:   "missing run id",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.RunID = "  "
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("RUN_ID_REQUIRED", "run id is required"),
		},
		{
			name:   "missing tenant",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.TenantID = ""
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name:   "negative timing window rejected",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.TimingWindow = -time.Hour
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("TIMING_WINDOW_INVALID", "timing window must be non-negative"),
		},
		{
			name:   "whitespace actor rejected",
			ledger: baseLedger,
			external: func() []entity.ExternalStatementLine {
				out := append([]entity.ExternalStatementLine(nil), baseExternal...)
				return out
			}(),
			cfg: func() service.MatchConfig {
				c := baseCfg
				c.Actor = "   "
				return c
			}(),
			expectedMatched: nil,
			expectedBreaks:  nil,
			expectedError:   entity.NewError("DECISION_ACTOR_REQUIRED", "match decision actor is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualMatched, actualBreaks, err := service.MatchStatements(tc.ledger, tc.external, tc.cfg)
			assert.Equal(t, tc.expectedMatched, actualMatched)
			assert.Equal(t, tc.expectedBreaks, actualBreaks)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestAutoResolveBreak(t *testing.T) {
	t.Parallel()

	baseBreak := entity.ReconciliationBreak{
		BreakID:     "brk-1",
		RunID:       "run-1",
		TenantID:    "t-1",
		Type:        valueobject.BreakDateMismatch,
		LedgerRef:   "p-1",
		ExternalRef: "e-1",
		RuleVersion: "v1",
	}
	basePolicy := service.AutoResolvePolicy{
		RuleVersion: "v1",
		MaxSkew:     24 * time.Hour,
		AllowedTypes: []valueobject.BreakType{
			valueobject.BreakDateMismatch,
			valueobject.BreakMissingInBank,
			valueobject.BreakMissingInLedger,
		},
		DecisionActor: "auto-engine",
	}

	type testCase struct {
		name           string
		b              entity.ReconciliationBreak
		skew           time.Duration
		policy         service.AutoResolvePolicy
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "timing skew inside policy resolves",
			b:              baseBreak,
			skew:           2 * time.Hour,
			policy:         basePolicy,
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name:           "skew beyond max does not resolve",
			b:              baseBreak,
			skew:           48 * time.Hour,
			policy:         basePolicy,
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "amount mismatch never resolves",
			b: func() entity.ReconciliationBreak {
				b := baseBreak
				b.Type = valueobject.BreakAmountMismatch
				return b
			}(),
			skew:           0,
			policy:         basePolicy,
			expectedResult: false,
			expectedError:  entity.NewError("AUTO_RESOLVE_FORBIDDEN", "amount mismatches require human review"),
		},
		{
			name: "duplicate never resolves",
			b: func() entity.ReconciliationBreak {
				b := baseBreak
				b.Type = valueobject.BreakDuplicate
				return b
			}(),
			skew:           0,
			policy:         basePolicy,
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "type not in allow list does not resolve",
			b:    baseBreak,
			skew: time.Hour,
			policy: func() service.AutoResolvePolicy {
				p := basePolicy
				p.AllowedTypes = []valueobject.BreakType{valueobject.BreakMissingInBank}
				return p
			}(),
			expectedResult: false,
			expectedError:  nil,
		},
		{
			name: "missing rule version errors",
			b:    baseBreak,
			skew: time.Hour,
			policy: func() service.AutoResolvePolicy {
				p := basePolicy
				p.RuleVersion = ""
				return p
			}(),
			expectedResult: false,
			expectedError:  entity.NewError("RULE_VERSION_REQUIRED", "auto-resolve requires a rule version"),
		},
		{
			name: "missing actor rejected",
			b:    baseBreak,
			skew: time.Hour,
			policy: func() service.AutoResolvePolicy {
				p := basePolicy
				p.DecisionActor = ""
				return p
			}(),
			expectedResult: false,
			expectedError:  entity.NewError("DECISION_ACTOR_REQUIRED", "auto-resolve requires a decision actor"),
		},
		{
			name: "whitespace actor rejected",
			b:    baseBreak,
			skew: time.Hour,
			policy: func() service.AutoResolvePolicy {
				p := basePolicy
				p.DecisionActor = "   "
				return p
			}(),
			expectedResult: false,
			expectedError:  entity.NewError("DECISION_ACTOR_REQUIRED", "auto-resolve requires a decision actor"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.AutoResolveBreak(tc.b, tc.skew, tc.policy)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestLedgerFactValidate(t *testing.T) {
	t.Parallel()

	baseTime := time.Date(2026, 9, 14, 2, 0, 0, 0, time.UTC)
	baseFact := service.LedgerFact{
		PostingID:   "p-1",
		Reference:   "ref-1",
		AmountMinor: 100,
		AssetCode:   "USD",
		EffectiveAt: baseTime,
	}

	type testCase struct {
		name          string
		fact          service.LedgerFact
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid fact",
			fact:          baseFact,
			expectedError: nil,
		},
		{
			name: "missing posting",
			fact: func() service.LedgerFact {
				f := baseFact
				f.PostingID = ""
				return f
			}(),
			expectedError: entity.NewError("POSTING_ID_REQUIRED", "ledger fact posting id is required"),
		},
		{
			name: "missing reference",
			fact: func() service.LedgerFact {
				f := baseFact
				f.Reference = "  "
				return f
			}(),
			expectedError: entity.NewError("LEDGER_REFERENCE_REQUIRED", "ledger fact reference is required"),
		},
		{
			name: "negative amount",
			fact: func() service.LedgerFact {
				f := baseFact
				f.AmountMinor = -1
				return f
			}(),
			expectedError: entity.NewError("INVALID_ENTRY_AMOUNT", "ledger fact amount must be positive"),
		},
		{
			name: "missing asset",
			fact: func() service.LedgerFact {
				f := baseFact
				f.AssetCode = ""
				return f
			}(),
			expectedError: entity.NewError("LEDGER_ASSET_REQUIRED", "ledger fact asset is required"),
		},
		{
			name: "missing time",
			fact: func() service.LedgerFact {
				f := baseFact
				f.EffectiveAt = time.Time{}
				return f
			}(),
			expectedError: entity.NewError("LEDGER_EFFECTIVE_REQUIRED", "ledger fact effective time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.fact.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
