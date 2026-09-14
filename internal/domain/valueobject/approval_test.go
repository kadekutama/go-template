package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseApproval(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.BreakStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "open",
			s:              "OPEN",
			expectedResult: valueobject.BreakOpen,
			expectedError:  nil,
		},
		{
			name:           "in review",
			s:              "IN_REVIEW",
			expectedResult: valueobject.BreakInReview,
			expectedError:  nil,
		},
		{
			name:           "resolved",
			s:              "RESOLVED",
			expectedResult: valueobject.BreakResolved,
			expectedError:  nil,
		},
		{
			name:           "acknowledged",
			s:              "ACKNOWLEDGED",
			expectedResult: valueobject.BreakAcknowledged,
			expectedError:  nil,
		},
		{
			name:           "escalated",
			s:              "ESCALATED",
			expectedResult: valueobject.BreakEscalated,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "PENDING",
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  errors.New(`reconciliation: invalid break status "PENDING"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  errors.New(`reconciliation: invalid break status ""`),
		},
		{
			name:           "whitespace status",
			s:              "   ",
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  errors.New(`reconciliation: invalid break status "   "`),
		},
		{
			name:           "lowercase status rejected",
			s:              "open",
			expectedResult: valueobject.BreakStatus(""),
			expectedError:  errors.New(`reconciliation: invalid break status "open"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParseBreakStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseResolutionAction(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.ResolutionAction
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "adjust ledger",
			s:              "ADJUST_LEDGER",
			expectedResult: valueobject.ResolveAdjustLedger,
			expectedError:  nil,
		},
		{
			name:           "external error",
			s:              "MARK_EXTERNAL_ERROR",
			expectedResult: valueobject.ResolveExternalError,
			expectedError:  nil,
		},
		{
			name:           "escalate",
			s:              "ESCALATE_TO_COMPLIANCE",
			expectedResult: valueobject.ResolveEscalate,
			expectedError:  nil,
		},
		{
			name:           "acknowledge",
			s:              "ACKNOWLEDGE",
			expectedResult: valueobject.ResolveAcknowledge,
			expectedError:  nil,
		},
		{
			name:           "invalid action",
			s:              "DELETE",
			expectedResult: valueobject.ResolutionAction(""),
			expectedError:  errors.New(`reconciliation: invalid resolution action "DELETE"`),
		},
		{
			name:           "empty action",
			s:              "",
			expectedResult: valueobject.ResolutionAction(""),
			expectedError:  errors.New(`reconciliation: invalid resolution action ""`),
		},
		{
			name:           "whitespace action",
			s:              "   ",
			expectedResult: valueobject.ResolutionAction(""),
			expectedError:  errors.New(`reconciliation: invalid resolution action "   "`),
		},
		{
			name:           "lowercase action rejected",
			s:              "acknowledge",
			expectedResult: valueobject.ResolutionAction(""),
			expectedError:  errors.New(`reconciliation: invalid resolution action "acknowledge"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParseResolutionAction(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionBreak(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.BreakStatus
		action         valueobject.ResolutionAction
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "open acknowledge allowed",
			from:           valueobject.BreakOpen,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: true,
		},
		{
			name:           "open adjust ledger allowed",
			from:           valueobject.BreakOpen,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: true,
		},
		{
			name:           "open external error allowed",
			from:           valueobject.BreakOpen,
			action:         valueobject.ResolveExternalError,
			expectedResult: true,
		},
		{
			name:           "open escalate allowed",
			from:           valueobject.BreakOpen,
			action:         valueobject.ResolveEscalate,
			expectedResult: true,
		},
		{
			name:           "in review adjust allowed",
			from:           valueobject.BreakInReview,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: true,
		},
		{
			name:           "in review external error allowed",
			from:           valueobject.BreakInReview,
			action:         valueobject.ResolveExternalError,
			expectedResult: true,
		},
		{
			name:           "in review escalate allowed",
			from:           valueobject.BreakInReview,
			action:         valueobject.ResolveEscalate,
			expectedResult: true,
		},
		{
			name:           "in review acknowledge allowed",
			from:           valueobject.BreakInReview,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: true,
		},
		{
			name:           "acknowledged frozen adjust forbidden",
			from:           valueobject.BreakAcknowledged,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: false,
		},
		{
			name:           "acknowledged frozen acknowledge forbidden",
			from:           valueobject.BreakAcknowledged,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: false,
		},
		{
			name:           "escalated adjust allowed",
			from:           valueobject.BreakEscalated,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: true,
		},
		{
			name:           "escalated external error allowed",
			from:           valueobject.BreakEscalated,
			action:         valueobject.ResolveExternalError,
			expectedResult: true,
		},
		{
			name:           "escalated acknowledge forbidden",
			from:           valueobject.BreakEscalated,
			action:         valueobject.ResolveAcknowledge,
			expectedResult: false,
		},
		{
			name:           "escalated escalate forbidden",
			from:           valueobject.BreakEscalated,
			action:         valueobject.ResolveEscalate,
			expectedResult: false,
		},
		{
			name:           "resolved frozen adjust forbidden",
			from:           valueobject.BreakResolved,
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: false,
		},
		{
			name:           "resolved frozen external error forbidden",
			from:           valueobject.BreakResolved,
			action:         valueobject.ResolveExternalError,
			expectedResult: false,
		},
		{
			name:           "unknown status forbidden",
			from:           valueobject.BreakStatus("UNKNOWN"),
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: false,
		},
		{
			name:           "empty status forbidden",
			from:           valueobject.BreakStatus(""),
			action:         valueobject.ResolveAdjustLedger,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult := valueobject.CanTransitionBreak(tc.from, tc.action)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
