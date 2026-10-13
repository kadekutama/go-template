package valueobject_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestValidateDescriptor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		s             string
		network       string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid standard descriptor",
			s:             "ACME CORP 123",
			network:       "visa",
			expectedError: nil,
		},
		{
			name:          "valid allowed punctuation",
			s:             "ACME-CORP. CO, *'&",
			network:       "mastercard",
			expectedError: nil,
		},
		{
			name:          "exact 22 characters",
			s:             "1234567890123456789012",
			network:       "amex",
			expectedError: nil,
		},
		{
			name:          "empty descriptor",
			s:             "",
			network:       "visa",
			expectedError: valueobject.ErrDescriptorValueIsRequired,
		},
		{
			name:          "exceeds 22 characters",
			s:             "12345678901234567890123",
			network:       "visa",
			expectedError: valueobject.ErrDescriptorINVALIDDESCRIPTORExceeds22Characters,
		},
		{
			name:          "control character null",
			s:             "ACME\x00CORP",
			network:       "visa",
			expectedError: valueobject.ErrDescriptorINVALIDDESCRIPTORControlCharacter,
		},
		{
			name:          "control character newline",
			s:             "ACME\nCORP",
			network:       "visa",
			expectedError: valueobject.ErrDescriptorINVALIDDESCRIPTORControlCharacter,
		},
		{
			name:          "control character del",
			s:             "ACME\x7fCORP",
			network:       "visa",
			expectedError: valueobject.ErrDescriptorINVALIDDESCRIPTORControlCharacter,
		},
		{
			name:          "illegal symbol exclamation",
			s:             "ACME! CORP",
			network:       "visa",
			expectedError: fmt.Errorf("descriptor: INVALID_DESCRIPTOR illegal character %q: %w", '!', valueobject.ErrDescriptorINVALIDDESCRIPTORIllegalCharacter),
		},
		{
			name:          "illegal symbol at",
			s:             "ACME@CORP",
			network:       "visa",
			expectedError: fmt.Errorf("descriptor: INVALID_DESCRIPTOR illegal character %q: %w", '@', valueobject.ErrDescriptorINVALIDDESCRIPTORIllegalCharacter),
		},
		{
			name:          "empty network",
			s:             "ACME CORP",
			network:       "",
			expectedError: valueobject.ErrDescriptorNetworkIsRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := valueobject.ValidateDescriptor(tc.s, tc.network)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
