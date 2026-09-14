package valueobject_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseAuthStatus(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.AuthStatus
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "authorized",
			s:              "AUTHORIZED",
			expectedResult: valueobject.AuthAuthorized,
			expectedError:  nil,
		},
		{
			name:           "partially captured",
			s:              "PARTIALLY_CAPTURED",
			expectedResult: valueobject.AuthPartiallyCaptured,
			expectedError:  nil,
		},
		{
			name:           "captured",
			s:              "CAPTURED",
			expectedResult: valueobject.AuthCaptured,
			expectedError:  nil,
		},
		{
			name:           "voided",
			s:              "VOIDED",
			expectedResult: valueobject.AuthVoided,
			expectedError:  nil,
		},
		{
			name:           "expired",
			s:              "EXPIRED",
			expectedResult: valueobject.AuthExpired,
			expectedError:  nil,
		},
		{
			name:           "requires action",
			s:              "REQUIRES_ACTION",
			expectedResult: valueobject.AuthRequiresAction,
			expectedError:  nil,
		},
		{
			name:           "invalid status",
			s:              "INVALID",
			expectedResult: valueobject.AuthStatus(""),
			expectedError:  errors.New(`auth: invalid status "INVALID"`),
		},
		{
			name:           "empty status",
			s:              "",
			expectedResult: valueobject.AuthStatus(""),
			expectedError:  errors.New(`auth: invalid status ""`),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult, err := valueobject.ParseAuthStatus(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestCanTransitionAuth(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		from           valueobject.AuthStatus
		to             valueobject.AuthStatus
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "authorized to partial",
			from:           valueobject.AuthAuthorized,
			to:             valueobject.AuthPartiallyCaptured,
			expectedResult: true,
		},
		{
			name:           "authorized to captured",
			from:           valueobject.AuthAuthorized,
			to:             valueobject.AuthCaptured,
			expectedResult: true,
		},
		{
			name:           "authorized to voided",
			from:           valueobject.AuthAuthorized,
			to:             valueobject.AuthVoided,
			expectedResult: true,
		},
		{
			name:           "authorized to expired",
			from:           valueobject.AuthAuthorized,
			to:             valueobject.AuthExpired,
			expectedResult: true,
		},
		{
			name:           "authorized to action",
			from:           valueobject.AuthAuthorized,
			to:             valueobject.AuthRequiresAction,
			expectedResult: true,
		},
		{
			name:           "partial to partial",
			from:           valueobject.AuthPartiallyCaptured,
			to:             valueobject.AuthPartiallyCaptured,
			expectedResult: true,
		},
		{
			name:           "partial to captured",
			from:           valueobject.AuthPartiallyCaptured,
			to:             valueobject.AuthCaptured,
			expectedResult: true,
		},
		{
			name:           "partial to voided",
			from:           valueobject.AuthPartiallyCaptured,
			to:             valueobject.AuthVoided,
			expectedResult: true,
		},
		{
			name:           "partial to expired",
			from:           valueobject.AuthPartiallyCaptured,
			to:             valueobject.AuthExpired,
			expectedResult: true,
		},
		{
			name:           "action to authorized",
			from:           valueobject.AuthRequiresAction,
			to:             valueobject.AuthAuthorized,
			expectedResult: true,
		},
		{
			name:           "action to voided",
			from:           valueobject.AuthRequiresAction,
			to:             valueobject.AuthVoided,
			expectedResult: true,
		},
		{
			name:           "captured to expired (illegal)",
			from:           valueobject.AuthCaptured,
			to:             valueobject.AuthExpired,
			expectedResult: false,
		},
		{
			name:           "voided to captured (illegal)",
			from:           valueobject.AuthVoided,
			to:             valueobject.AuthCaptured,
			expectedResult: false,
		},
		{
			name:           "expired to authorized (illegal)",
			from:           valueobject.AuthExpired,
			to:             valueobject.AuthAuthorized,
			expectedResult: false,
		},
		{
			name:           "unknown from (illegal)",
			from:           valueobject.AuthStatus("UNKNOWN"),
			to:             valueobject.AuthAuthorized,
			expectedResult: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actualResult := valueobject.CanTransitionAuth(tc.from, tc.to)
			assert.Equal(t, tc.expectedResult, actualResult)
		})
	}
}
