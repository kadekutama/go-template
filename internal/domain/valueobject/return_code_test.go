package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestReturnCodes(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		expectedResult []string
	}

	testCases := []testCase{
		{
			name: "full return code catalog",
			expectedResult: []string{
				"CARD_DECLINED", "CARD_EXPIRED", "CARD_FRAUD_SUSPECTED", "CARD_INSUFFICIENT", "CARD_INVALID",
				"R01", "R02", "R03", "R04", "R06", "R07", "R08", "R09",
				"R10", "R11", "R12", "R13", "R14", "R15", "R16", "R17",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := valueobject.ReturnCodes()
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}

func TestDispositionFor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		code           string
		expectedResult valueobject.ReturnDisposition
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "R01 auto-reversal",
			code:           "R01",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "R02 auto-reversal",
			code:           "R02",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "R03 manual review",
			code:           "R03",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R04 manual review",
			code:           "R04",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R06 auto-reversal",
			code:           "R06",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "R07 manual review",
			code:           "R07",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R08 manual review",
			code:           "R08",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R09 auto-reversal",
			code:           "R09",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "R10 manual review",
			code:           "R10",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R11 manual review",
			code:           "R11",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R12 auto-reversal",
			code:           "R12",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "R13 manual review",
			code:           "R13",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R14 manual review",
			code:           "R14",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R15 manual review",
			code:           "R15",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R16 manual review",
			code:           "R16",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "R17 manual review",
			code:           "R17",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card declined auto-reversal",
			code:           "CARD_DECLINED",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "card insufficient auto-reversal",
			code:           "CARD_INSUFFICIENT",
			expectedResult: valueobject.ReturnAutoReversal,
			expectedError:  nil,
		},
		{
			name:           "card fraud suspected manual review",
			code:           "CARD_FRAUD_SUSPECTED",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card expired manual review",
			code:           "CARD_EXPIRED",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "card invalid manual review",
			code:           "CARD_INVALID",
			expectedResult: valueobject.ReturnManualReview,
			expectedError:  nil,
		},
		{
			name:           "unknown code error",
			code:           "UNKNOWN_CODE",
			expectedResult: valueobject.ReturnDisposition(""),
			expectedError:  errors.New(`payment: unknown return code "UNKNOWN_CODE"`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := valueobject.DispositionFor(tc.code)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
