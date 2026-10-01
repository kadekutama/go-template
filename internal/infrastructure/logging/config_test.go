package logging_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
)

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cfg           logging.Config
		expectedError error
	}

	testCases := []testCase{
		{
			name: "valid debug level",
			cfg: logging.Config{
				Level: "debug",
			},
			expectedError: nil,
		},
		{
			name: "valid info level",
			cfg: logging.Config{
				Level: "info",
			},
			expectedError: nil,
		},
		{
			name: "valid warn level",
			cfg: logging.Config{
				Level: "warn",
			},
			expectedError: nil,
		},
		{
			name: "valid error level",
			cfg: logging.Config{
				Level: "error",
			},
			expectedError: nil,
		},
		{
			name: "disabled silences output for tests and quiet workers",
			cfg: logging.Config{
				Level: "disabled",
			},
			expectedError: nil,
		},
		{
			name: "empty level rejected",
			cfg: logging.Config{
				Level: "",
			},
			expectedError: errors.New("logging: level is required"),
		},
		{
			name: "whitespace level rejected",
			cfg: logging.Config{
				Level: "   ",
			},
			expectedError: errors.New("logging: level is required"),
		},
		{
			name: "unknown level rejected",
			cfg: logging.Config{
				Level: "trace",
			},
			expectedError: errors.New("logging: unknown log level \"trace\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
