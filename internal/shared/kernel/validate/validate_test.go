package validate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

func TestStruct(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        any
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid struct passes",
			params: struct {
				Addr string        `validate:"required"`
				TTL  time.Duration `validate:"omitempty,gt=0"`
			}{Addr: "127.0.0.1:6379", TTL: time.Minute},
			expectedError: nil,
		},
		{
			name: "single violation joined",
			params: struct {
				Addr string `validate:"required"`
			}{},
			expectedError: errors.New(`cache: invalid params (1 violation(s)): Addr: rule "required" on value `),
		},
		{
			name: "multiple violations joined in one error",
			params: struct {
				Addr string        `validate:"required"`
				TTL  time.Duration `validate:"gt=0"`
			}{Addr: "", TTL: -time.Second},
			expectedError: errors.New(`cache: invalid params (2 violation(s)): Addr: rule "required" on value ; TTL: rule "gt" on value -1s`),
		},
		{
			name:          "non-struct input is an error",
			params:        "not-a-struct",
			expectedError: errors.New("cache: validate params"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate.Struct("cache", "params", tc.params)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
