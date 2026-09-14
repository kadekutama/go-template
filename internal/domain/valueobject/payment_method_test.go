package valueobject_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParsePaymentMethod(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		s              string
		expectedResult valueobject.PaymentMethod
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "ach",
			s:              "ACH",
			expectedResult: valueobject.MethodACH,
			expectedError:  nil,
		},
		{
			name:           "wire",
			s:              "WIRE",
			expectedResult: valueobject.MethodWire,
			expectedError:  nil,
		},
		{
			name:           "rtp",
			s:              "RTP",
			expectedResult: valueobject.MethodRTP,
			expectedError:  nil,
		},
		{
			name:           "card",
			s:              "CARD",
			expectedResult: valueobject.MethodCard,
			expectedError:  nil,
		},
		{
			name:           "crypto",
			s:              "CRYPTO",
			expectedResult: valueobject.MethodCrypto,
			expectedError:  nil,
		},
		{
			name:           "wallet",
			s:              "WALLET",
			expectedResult: valueobject.MethodWallet,
			expectedError:  nil,
		},
		{
			name:           "unknown method",
			s:              "CASH",
			expectedResult: valueobject.PaymentMethod(""),
			expectedError:  errors.New(`payment: invalid method "CASH"`),
		},
		{
			name:           "empty method",
			s:              "",
			expectedResult: valueobject.PaymentMethod(""),
			expectedError:  errors.New(`payment: invalid method ""`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.ParsePaymentMethod(tc.s)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestMethodCapabilities(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		m              valueobject.PaymentMethod
		expectedResult valueobject.Capabilities
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "ach capabilities",
			m:    valueobject.MethodACH,
			expectedResult: valueobject.Capabilities{
				Push:       true,
				Pull:       true,
				Lag:        48 * time.Hour,
				Reversible: true,
			},
			expectedError: nil,
		},
		{
			name: "wire capabilities",
			m:    valueobject.MethodWire,
			expectedResult: valueobject.Capabilities{
				Push:       true,
				Pull:       false,
				Lag:        12 * time.Hour,
				Reversible: false,
			},
			expectedError: nil,
		},
		{
			name: "rtp capabilities",
			m:    valueobject.MethodRTP,
			expectedResult: valueobject.Capabilities{
				Push:       true,
				Pull:       false,
				Lag:        15 * time.Minute,
				Reversible: false,
			},
			expectedError: nil,
		},
		{
			name: "card capabilities",
			m:    valueobject.MethodCard,
			expectedResult: valueobject.Capabilities{
				Push:       false,
				Pull:       true,
				Lag:        time.Hour,
				Reversible: true,
			},
			expectedError: nil,
		},
		{
			name: "crypto capabilities",
			m:    valueobject.MethodCrypto,
			expectedResult: valueobject.Capabilities{
				Push:       true,
				Pull:       false,
				Lag:        time.Hour,
				Reversible: false,
			},
			expectedError: nil,
		},
		{
			name: "wallet capabilities",
			m:    valueobject.MethodWallet,
			expectedResult: valueobject.Capabilities{
				Push:       true,
				Pull:       true,
				Lag:        time.Hour,
				Reversible: true,
			},
			expectedError: nil,
		},
		{
			name:           "invalid method",
			m:              valueobject.PaymentMethod("UNKNOWN"),
			expectedResult: valueobject.Capabilities{},
			expectedError:  errors.New(`payment: invalid method "UNKNOWN"`),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := valueobject.MethodCapabilities(tc.m)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
