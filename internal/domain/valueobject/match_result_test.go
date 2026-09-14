package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseMatchDecision(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.MatchDecision
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "matched",
			s:              "MATCHED",
			expectedResult: valueobject.MatchMatched,
			expectedError:  nil,
		},
		{
			name:           "break",
			s:              "BREAK",
			expectedResult: valueobject.MatchBreak,
			expectedError:  nil,
		},
		{
			name:           "invalid decision",
			s:              "PARTIAL",
			expectedResult: valueobject.MatchDecision(""),
			expectedError:  errors.New(`reconciliation: invalid match decision "PARTIAL"`),
		},
		{
			name:           "empty decision",
			s:              "",
			expectedResult: valueobject.MatchDecision(""),
			expectedError:  errors.New(`reconciliation: invalid match decision ""`),
		},
		{
			name:           "whitespace decision",
			s:              "   ",
			expectedResult: valueobject.MatchDecision(""),
			expectedError:  errors.New(`reconciliation: invalid match decision "   "`),
		},
		{
			name:           "lowercase decision rejected",
			s:              "matched",
			expectedResult: valueobject.MatchDecision(""),
			expectedError:  errors.New(`reconciliation: invalid match decision "matched"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParseMatchDecision(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
