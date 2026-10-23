package logging_test

import (
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
			expectedError: logging.ErrLoggingLevelIsRequired,
		},
		{
			name: "whitespace level rejected",
			cfg: logging.Config{
				Level: "   ",
			},
			expectedError: logging.ErrLoggingLevelIsRequired,
		},
		{
			name: "unknown level rejected",
			cfg: logging.Config{
				Level: "trace",
			},
			expectedError: logging.ErrLoggingUnknownLogLevelTrace,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())

				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestConfigValidateOutputModes(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cfg           logging.Config
		expectedError error
	}

	testCases := []testCase{
		{
			name: "empty output defaults to stdout",
			cfg: logging.Config{
				Level: "info",
			},
			expectedError: nil,
		},
		{
			name: "stdout output is accepted and normalized",
			cfg: logging.Config{
				Level:  "info",
				Output: "  STDOUT  ",
			},
			expectedError: nil,
		},
		{
			name: "file output with path and positive size is accepted",
			cfg: logging.Config{
				Level:     "info",
				Output:    logging.OutputFile,
				FilePath:  "/var/log/go-template/app.log",
				MaxSizeMB: 100,
			},
			expectedError: nil,
		},
		{
			name: "both output with path and positive size is accepted",
			cfg: logging.Config{
				Level:     "info",
				Output:    logging.OutputBoth,
				FilePath:  "/var/log/go-template/app.log",
				MaxSizeMB: 100,
			},
			expectedError: nil,
		},
		{
			name: "file output requires a positive max size",
			cfg: logging.Config{
				Level:    "info",
				Output:   logging.OutputFile,
				FilePath: "/var/log/go-template/app.log",
			},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "both output requires a positive max size",
			cfg: logging.Config{
				Level:    "info",
				Output:   logging.OutputBoth,
				FilePath: "/var/log/go-template/app.log",
			},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "unknown output is rejected",
			cfg: logging.Config{
				Level:  "info",
				Output: "syslog",
			},
			expectedError: logging.ErrLoggingUnknownOutputMode,
		},
		{
			name: "file output requires a path",
			cfg: logging.Config{
				Level:  "info",
				Output: logging.OutputFile,
			},
			expectedError: logging.ErrLoggingFilePathIsRequired,
		},
		{
			name: "both output requires a path",
			cfg: logging.Config{
				Level:  "info",
				Output: logging.OutputBoth,
			},
			expectedError: logging.ErrLoggingFilePathIsRequired,
		},
		{
			name: "negative max size is rejected",
			cfg: logging.Config{
				Level:     "info",
				Output:    logging.OutputFile,
				FilePath:  "/var/log/go-template/app.log",
				MaxSizeMB: -1,
			},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "negative max backups is rejected",
			cfg: logging.Config{
				Level:      "info",
				Output:     logging.OutputFile,
				FilePath:   "/var/log/go-template/app.log",
				MaxBackups: -1,
			},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "negative max age is rejected",
			cfg: logging.Config{
				Level:      "info",
				Output:     logging.OutputFile,
				FilePath:   "/var/log/go-template/app.log",
				MaxAgeDays: -1,
			},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := tc.cfg.Validate()
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}
