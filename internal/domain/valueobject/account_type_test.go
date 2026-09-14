package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseDirection(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		raw            string
		expectedResult valueobject.Direction
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid debit",
			raw:            "DEBIT",
			expectedResult: valueobject.DirectionDebit,
			expectedError:  nil,
		},
		{
			name:           "valid credit",
			raw:            "CREDIT",
			expectedResult: valueobject.DirectionCredit,
			expectedError:  nil,
		},
		{
			name:           "invalid direction",
			raw:            "SIDEWAYS",
			expectedResult: "",
			expectedError:  errors.New("account: invalid direction \"SIDEWAYS\""),
		},
		{
			name:           "empty string",
			raw:            "",
			expectedResult: "",
			expectedError:  errors.New("account: invalid direction \"\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobject.ParseDirection(tc.raw)
			assert.Equal(t, tc.expectedResult, got)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseAccountClass(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		raw            string
		expectedResult valueobject.AccountClass
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid asset",
			raw:            "ASSET",
			expectedResult: valueobject.ClassAsset,
			expectedError:  nil,
		},
		{
			name:           "valid liability",
			raw:            "LIABILITY",
			expectedResult: valueobject.ClassLiability,
			expectedError:  nil,
		},
		{
			name:           "valid equity",
			raw:            "EQUITY",
			expectedResult: valueobject.ClassEquity,
			expectedError:  nil,
		},
		{
			name:           "valid revenue",
			raw:            "REVENUE",
			expectedResult: valueobject.ClassRevenue,
			expectedError:  nil,
		},
		{
			name:           "valid expense",
			raw:            "EXPENSE",
			expectedResult: valueobject.ClassExpense,
			expectedError:  nil,
		},
		{
			name:           "invalid class",
			raw:            "CRYPTO",
			expectedResult: "",
			expectedError:  errors.New("account: invalid class \"CRYPTO\""),
		},
		{
			name:           "empty class",
			raw:            "",
			expectedResult: "",
			expectedError:  errors.New("account: invalid class \"\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobject.ParseAccountClass(tc.raw)
			assert.Equal(t, tc.expectedResult, got)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNormalSide(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		class          valueobject.AccountClass
		expectedResult valueobject.Direction
	}

	testCases := []testCase{
		{
			name:           "asset normal side is debit",
			class:          valueobject.ClassAsset,
			expectedResult: valueobject.DirectionDebit,
		},
		{
			name:           "expense normal side is debit",
			class:          valueobject.ClassExpense,
			expectedResult: valueobject.DirectionDebit,
		},
		{
			name:           "liability normal side is credit",
			class:          valueobject.ClassLiability,
			expectedResult: valueobject.DirectionCredit,
		},
		{
			name:           "equity normal side is credit",
			class:          valueobject.ClassEquity,
			expectedResult: valueobject.DirectionCredit,
		},
		{
			name:           "revenue normal side is credit",
			class:          valueobject.ClassRevenue,
			expectedResult: valueobject.DirectionCredit,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.class.NormalSide()
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}

func TestParseAccountStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		raw            string
		expectedResult valueobject.AccountStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid active",
			raw:            "ACTIVE",
			expectedResult: valueobject.StatusActive,
			expectedError:  nil,
		},
		{
			name:           "valid frozen",
			raw:            "FROZEN",
			expectedResult: valueobject.StatusFrozen,
			expectedError:  nil,
		},
		{
			name:           "valid closed",
			raw:            "CLOSED",
			expectedResult: valueobject.StatusClosed,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			raw:            "PENDING",
			expectedResult: "",
			expectedError:  errors.New("account: invalid status \"PENDING\""),
		},
		{
			name:           "empty status",
			raw:            "",
			expectedResult: "",
			expectedError:  errors.New("account: invalid status \"\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := valueobject.ParseAccountStatus(tc.raw)
			assert.Equal(t, tc.expectedResult, got)
			if tc.expectedError != nil {
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
