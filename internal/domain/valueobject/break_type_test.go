package valueobject_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseBreakType(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.BreakType
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "missing in ledger",
			s:              "MISSING_IN_LEDGER",
			expectedResult: valueobject.BreakMissingInLedger,
			expectedError:  nil,
		},
		{
			name:           "missing in bank",
			s:              "MISSING_IN_BANK",
			expectedResult: valueobject.BreakMissingInBank,
			expectedError:  nil,
		},
		{
			name:           "amount mismatch",
			s:              "AMOUNT_MISMATCH",
			expectedResult: valueobject.BreakAmountMismatch,
			expectedError:  nil,
		},
		{
			name:           "date mismatch",
			s:              "DATE_MISMATCH",
			expectedResult: valueobject.BreakDateMismatch,
			expectedError:  nil,
		},
		{
			name:           "duplicate",
			s:              "DUPLICATE",
			expectedResult: valueobject.BreakDuplicate,
			expectedError:  nil,
		},
		{
			name:           "invalid type",
			s:              "UNKNOWN",
			expectedResult: valueobject.BreakType(""),
			expectedError:  fmt.Errorf("reconciliation: invalid break type %q: %w", "UNKNOWN", valueobject.ErrReconciliationInvalidBreakType),
		},
		{
			name:           "empty type",
			s:              "",
			expectedResult: valueobject.BreakType(""),
			expectedError:  fmt.Errorf("reconciliation: invalid break type %q: %w", "", valueobject.ErrReconciliationInvalidBreakType),
		},
		{
			name:           "lowercase rejected",
			s:              "duplicate",
			expectedResult: valueobject.BreakType(""),
			expectedError:  fmt.Errorf("reconciliation: invalid break type %q: %w", "duplicate", valueobject.ErrReconciliationInvalidBreakType),
		},
		{
			name:           "whitespace type",
			s:              "   ",
			expectedResult: valueobject.BreakType(""),
			expectedError:  fmt.Errorf("reconciliation: invalid break type %q: %w", "   ", valueobject.ErrReconciliationInvalidBreakType),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult, err := valueobject.ParseBreakType(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
