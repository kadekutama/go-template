package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestReconciliationBreakValidate(t *testing.T) {
	t.Parallel()

	baseBreak := entity.ReconciliationBreak{
		BreakID:       "brk-100",
		RunID:         "run-200",
		TenantID:      "tenant-300",
		Type:          valueobject.BreakMissingInLedger,
		LedgerRef:     "",
		ExternalRef:   "ext-line-1",
		ExpectedMinor: 0,
		ActualMinor:   5000,
		Evidence: map[string]string{
			"reference": "ref-xyz",
		},
		RuleVersion: "v1",
	}

	type testCase struct {
		name          string
		b             entity.ReconciliationBreak
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid missing in ledger",
			b:             baseBreak,
			expectedError: nil,
		},
		{
			name: "valid missing in bank",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakMissingInBank
				brk.LedgerRef = "post-1"
				brk.ExternalRef = ""
				return brk
			}(),
			expectedError: nil,
		},
		{
			name: "valid amount mismatch",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakAmountMismatch
				brk.LedgerRef = "post-1"
				brk.ExternalRef = "ext-line-1"
				return brk
			}(),
			expectedError: nil,
		},
		{
			name: "valid date mismatch",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDateMismatch
				brk.LedgerRef = "post-1"
				brk.ExternalRef = "ext-line-1"
				return brk
			}(),
			expectedError: nil,
		},
		{
			name: "valid duplicate",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDuplicate
				brk.ExternalRef = "ext-line-2"
				brk.Evidence = map[string]string{
					"duplicate_of": "ref-xyz",
				}
				return brk
			}(),
			expectedError: nil,
		},
		{
			name: "missing break id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.BreakID = ""
				return brk
			}(),
			expectedError: entity.NewError("BREAK_ID_REQUIRED", "break id is required"),
		},
		{
			name: "whitespace break id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.BreakID = "   "
				return brk
			}(),
			expectedError: entity.NewError("BREAK_ID_REQUIRED", "break id is required"),
		},
		{
			name: "missing run id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.RunID = ""
				return brk
			}(),
			expectedError: entity.NewError("RUN_ID_REQUIRED", "run id is required"),
		},
		{
			name: "whitespace run id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.RunID = "   "
				return brk
			}(),
			expectedError: entity.NewError("RUN_ID_REQUIRED", "run id is required"),
		},
		{
			name: "missing tenant id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.TenantID = ""
				return brk
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "whitespace tenant id",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.TenantID = "   "
				return brk
			}(),
			expectedError: entity.NewError("TENANT_REQUIRED", "tenant id is required"),
		},
		{
			name: "invalid break type",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakType("INVALID_TYPE")
				return brk
			}(),
			expectedError: entity.NewError("BREAK_TYPE_INVALID", "break type is invalid"),
		},
		{
			name: "empty break type",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakType("")
				return brk
			}(),
			expectedError: entity.NewError("BREAK_TYPE_INVALID", "break type is invalid"),
		},
		{
			name: "missing rule version",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.RuleVersion = ""
				return brk
			}(),
			expectedError: entity.NewError("RULE_VERSION_REQUIRED", "rule version is required"),
		},
		{
			name: "whitespace rule version",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.RuleVersion = "   "
				return brk
			}(),
			expectedError: entity.NewError("RULE_VERSION_REQUIRED", "rule version is required"),
		},
		{
			name: "missing in ledger missing external ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.ExternalRef = ""
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-ledger requires the external reference"),
		},
		{
			name: "missing in ledger whitespace external ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.ExternalRef = "   "
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-ledger requires the external reference"),
		},
		{
			name: "missing in bank missing ledger ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakMissingInBank
				brk.LedgerRef = ""
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-bank requires the ledger reference"),
		},
		{
			name: "missing in bank whitespace ledger ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakMissingInBank
				brk.LedgerRef = "   "
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "missing-in-bank requires the ledger reference"),
		},
		{
			name: "amount mismatch missing ledger ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakAmountMismatch
				brk.LedgerRef = ""
				brk.ExternalRef = "ext-1"
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "mismatch requires both references"),
		},
		{
			name: "amount mismatch missing external ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakAmountMismatch
				brk.LedgerRef = "post-1"
				brk.ExternalRef = ""
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "mismatch requires both references"),
		},
		{
			name: "date mismatch missing external ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDateMismatch
				brk.LedgerRef = "post-1"
				brk.ExternalRef = ""
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "mismatch requires both references"),
		},
		{
			name: "duplicate missing external ref",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDuplicate
				brk.ExternalRef = ""
				brk.Evidence = map[string]string{"duplicate_of": "ref-1"}
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires the external reference"),
		},
		{
			name: "duplicate nil evidence",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDuplicate
				brk.ExternalRef = "ext-1"
				brk.Evidence = nil
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires duplicate_of evidence"),
		},
		{
			name: "duplicate missing duplicate_of",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDuplicate
				brk.ExternalRef = "ext-1"
				brk.Evidence = map[string]string{}
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires duplicate_of evidence"),
		},
		{
			name: "duplicate whitespace duplicate_of",
			b: func() entity.ReconciliationBreak {
				brk := baseBreak
				brk.Type = valueobject.BreakDuplicate
				brk.ExternalRef = "ext-1"
				brk.Evidence = map[string]string{"duplicate_of": "   "}
				return brk
			}(),
			expectedError: entity.NewError("BREAK_EVIDENCE_REQUIRED", "duplicate requires duplicate_of evidence"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.b.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
