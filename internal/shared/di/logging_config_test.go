package di

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
)

// TestProvideLoggingConfigEnvironment exercises process-wide environment
// resolution. The parent and subtests intentionally remain sequential because
// t.Setenv cannot be used from parallel tests.
func TestProvideLoggingConfigEnvironment(t *testing.T) {
	type testCase struct {
		name          string
		env           map[string]string
		expected      logging.Config
		expectedError error
	}

	testCases := []testCase{
		{
			name: "local default keeps stdout visible",
			env:  map[string]string{},
			expected: logging.Config{
				Level:      "info",
				Output:     logging.OutputStdout,
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
			expectedError: nil,
		},
		{
			name: "staging defaults to a rotating file",
			env: map[string]string{
				"APP_ENV":      "staging",
				"APP_LOG_FILE": "/var/log/go-template/app.log",
			},
			expected: logging.Config{
				Level:      "info",
				Output:     logging.OutputFile,
				FilePath:   "/var/log/go-template/app.log",
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
			expectedError: nil,
		},
		{
			name: "explicit both and custom rotation",
			env: map[string]string{
				"APP_ENV":              "production",
				"APP_LOG_OUTPUT":       "both",
				"APP_LOG_FILE":         "/var/log/go-template/app.log",
				"APP_LOG_MAX_SIZE_MB":  "50",
				"APP_LOG_MAX_BACKUPS":  "3",
				"APP_LOG_MAX_AGE_DAYS": "14",
				"APP_LOG_COMPRESS":     "false",
			},
			expected: logging.Config{
				Level:      "info",
				Output:     logging.OutputBoth,
				FilePath:   "/var/log/go-template/app.log",
				MaxSizeMB:  50,
				MaxBackups: 3,
				MaxAgeDays: 14,
				Compress:   false,
			},
			expectedError: nil,
		},
		{
			name: "explicit stdout overrides production default",
			env: map[string]string{
				"APP_ENV":        "production",
				"APP_LOG_OUTPUT": "stdout",
			},
			expected: logging.Config{
				Level:      "info",
				Output:     logging.OutputStdout,
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
			expectedError: nil,
		},
		{
			name: "koanf fallbacks resolve server file output",
			env: map[string]string{
				"APP_APP__ENV":                  "production",
				"APP_OBSERVABILITY__LOG_OUTPUT": "file",
				"APP_OBSERVABILITY__LOG_FILE":   "/var/log/fallback.log",
			},
			expected: logging.Config{
				Level:      "info",
				Output:     logging.OutputFile,
				FilePath:   "/var/log/fallback.log",
				MaxSizeMB:  100,
				MaxBackups: 7,
				MaxAgeDays: 30,
				Compress:   true,
			},
			expectedError: nil,
		},
		{
			name: "staging file mode requires a path",
			env: map[string]string{
				"APP_ENV": "staging",
			},
			expected:      logging.Config{},
			expectedError: logging.ErrLoggingFilePathIsRequired,
		},
		{
			name: "unknown environment is rejected",
			env: map[string]string{
				"APP_ENV": "development",
			},
			expected:      logging.Config{},
			expectedError: ErrLoggingInvalidEnvironment,
		},
		{
			name: "unknown output is rejected",
			env: map[string]string{
				"APP_LOG_OUTPUT": "syslog",
			},
			expected:      logging.Config{},
			expectedError: logging.ErrLoggingUnknownOutputMode,
		},
		{
			name: "negative rotation value is rejected",
			env: map[string]string{
				"APP_LOG_OUTPUT":      "file",
				"APP_LOG_FILE":        "/var/log/go-template/app.log",
				"APP_LOG_MAX_SIZE_MB": "-1",
			},
			expected:      logging.Config{},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "zero max size is rejected in file mode",
			env: map[string]string{
				"APP_LOG_OUTPUT":      "file",
				"APP_LOG_FILE":        "/var/log/go-template/app.log",
				"APP_LOG_MAX_SIZE_MB": "0",
			},
			expected:      logging.Config{},
			expectedError: logging.ErrLoggingInvalidRotation,
		},
		{
			name: "malformed integer is rejected",
			env: map[string]string{
				"APP_LOG_MAX_BACKUPS": "many",
			},
			expected:      logging.Config{},
			expectedError: ErrLoggingInvalidEnvironmentValue,
		},
		{
			name: "malformed boolean is rejected",
			env: map[string]string{
				"APP_LOG_COMPRESS": "sometimes",
			},
			expected:      logging.Config{},
			expectedError: ErrLoggingInvalidEnvironmentValue,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clearLoggingEnv(t)
			t.Setenv("APP_LOG_LEVEL", "info")

			for key, value := range tc.env {
				t.Setenv(key, value)
			}

			cfg, err := ProvideLoggingConfig()
			if tc.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.expectedError)
			}

			assert.Equal(t, tc.expected, cfg)
		})
	}
}

func clearLoggingEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{
		"APP_ENV",
		"APP_APP__ENV",
		"APP_LOG_LEVEL",
		"APP_OBSERVABILITY__LOG_LEVEL",
		"APP_LOG_OUTPUT",
		"APP_OBSERVABILITY__LOG_OUTPUT",
		"APP_LOG_FILE",
		"APP_OBSERVABILITY__LOG_FILE",
		"APP_LOG_MAX_SIZE_MB",
		"APP_OBSERVABILITY__LOG_MAX_SIZE_MB",
		"APP_LOG_MAX_BACKUPS",
		"APP_OBSERVABILITY__LOG_MAX_BACKUPS",
		"APP_LOG_MAX_AGE_DAYS",
		"APP_OBSERVABILITY__LOG_MAX_AGE_DAYS",
		"APP_LOG_COMPRESS",
		"APP_OBSERVABILITY__LOG_COMPRESS",
	} {
		t.Setenv(key, "")
	}
}
