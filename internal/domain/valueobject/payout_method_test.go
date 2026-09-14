package valueobject_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParsePayoutMethod(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.PayoutMethod
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "rtp",
			s:              "RTP",
			expectedResult: valueobject.PayoutRTP,
			expectedError:  nil,
		},
		{
			name:           "fednow",
			s:              "FEDNOW",
			expectedResult: valueobject.PayoutFedNow,
			expectedError:  nil,
		},
		{
			name:           "ach",
			s:              "ACH",
			expectedResult: valueobject.PayoutACH,
			expectedError:  nil,
		},
		{
			name:           "wire",
			s:              "WIRE",
			expectedResult: valueobject.PayoutWire,
			expectedError:  nil,
		},
		{
			name:           "check",
			s:              "CHECK",
			expectedResult: valueobject.PayoutCheck,
			expectedError:  nil,
		},
		{
			name:           "unknown method",
			s:              "CARD",
			expectedResult: valueobject.PayoutMethod(""),
			expectedError:  errors.New(`payout: invalid method "CARD"`),
		},
		{
			name:           "empty method",
			s:              "",
			expectedResult: valueobject.PayoutMethod(""),
			expectedError:  errors.New(`payout: invalid method ""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParsePayoutMethod(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestExpectedSettlementLag(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		m              valueobject.PayoutMethod
		expectedResult time.Duration
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "rtp lag",
			m:              valueobject.PayoutRTP,
			expectedResult: 15 * time.Minute,
			expectedError:  nil,
		},
		{
			name:           "fednow lag",
			m:              valueobject.PayoutFedNow,
			expectedResult: 15 * time.Minute,
			expectedError:  nil,
		},
		{
			name:           "ach lag",
			m:              valueobject.PayoutACH,
			expectedResult: 48 * time.Hour,
			expectedError:  nil,
		},
		{
			name:           "wire lag",
			m:              valueobject.PayoutWire,
			expectedResult: 12 * time.Hour,
			expectedError:  nil,
		},
		{
			name:           "check lag",
			m:              valueobject.PayoutCheck,
			expectedResult: 120 * time.Hour,
			expectedError:  nil,
		},
		{
			name:           "invalid method lag",
			m:              valueobject.PayoutMethod("UNKNOWN"),
			expectedResult: 0,
			expectedError:  errors.New(`payout: invalid method "UNKNOWN"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ExpectedSettlementLag(tc.m)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
