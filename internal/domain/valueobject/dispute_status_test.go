package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseDisputeStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.DisputeStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "open",
			s:              "OPEN",
			expectedResult: valueobject.DisputeOpen,
			expectedError:  nil,
		},
		{
			name:           "evidence due",
			s:              "EVIDENCE_DUE",
			expectedResult: valueobject.DisputeEvidenceDue,
			expectedError:  nil,
		},
		{
			name:           "under review",
			s:              "UNDER_REVIEW",
			expectedResult: valueobject.DisputeUnderReview,
			expectedError:  nil,
		},
		{
			name:           "won",
			s:              "WON",
			expectedResult: valueobject.DisputeWon,
			expectedError:  nil,
		},
		{
			name:           "lost",
			s:              "LOST",
			expectedResult: valueobject.DisputeLost,
			expectedError:  nil,
		},
		{
			name:           "closed",
			s:              "CLOSED",
			expectedResult: valueobject.DisputeClosed,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "PENDING",
			expectedResult: valueobject.DisputeStatus(""),
			expectedError:  errors.New(`dispute: invalid status "PENDING"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.DisputeStatus(""),
			expectedError:  errors.New(`dispute: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := valueobject.ParseDisputeStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionDispute(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.DisputeStatus
		to             valueobject.DisputeStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "open to evidence due",
			from:           valueobject.DisputeOpen,
			to:             valueobject.DisputeEvidenceDue,
			expectedResult: true,
		},
		{
			name:           "open to under review",
			from:           valueobject.DisputeOpen,
			to:             valueobject.DisputeUnderReview,
			expectedResult: true,
		},
		{
			name:           "evidence due to under review",
			from:           valueobject.DisputeEvidenceDue,
			to:             valueobject.DisputeUnderReview,
			expectedResult: true,
		},
		{
			name:           "evidence due to closed",
			from:           valueobject.DisputeEvidenceDue,
			to:             valueobject.DisputeClosed,
			expectedResult: true,
		},
		{
			name:           "under review to won",
			from:           valueobject.DisputeUnderReview,
			to:             valueobject.DisputeWon,
			expectedResult: true,
		},
		{
			name:           "under review to lost",
			from:           valueobject.DisputeUnderReview,
			to:             valueobject.DisputeLost,
			expectedResult: true,
		},
		{
			name:           "won to closed",
			from:           valueobject.DisputeWon,
			to:             valueobject.DisputeClosed,
			expectedResult: true,
		},
		{
			name:           "lost to closed",
			from:           valueobject.DisputeLost,
			to:             valueobject.DisputeClosed,
			expectedResult: true,
		},
		{
			name:           "open to won (illegal)",
			from:           valueobject.DisputeOpen,
			to:             valueobject.DisputeWon,
			expectedResult: false,
		},
		{
			name:           "closed to open (illegal)",
			from:           valueobject.DisputeClosed,
			to:             valueobject.DisputeOpen,
			expectedResult: false,
		},
		{
			name:           "unknown from (illegal)",
			from:           valueobject.DisputeStatus("UNKNOWN"),
			to:             valueobject.DisputeClosed,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := valueobject.CanTransitionDispute(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
