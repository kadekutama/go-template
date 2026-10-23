package logging_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
)

func TestNewFromConfig(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		cfg               logging.Config
		expectedLoggerNil bool
		expectedError     error
	}

	testCases := []testCase{
		{
			name: "stdout mode returns a logger",
			cfg: logging.Config{
				Level:  "info",
				Output: logging.OutputStdout,
			},
			expectedLoggerNil: false,
			expectedError:     nil,
		},
		{
			name: "file mode creates directories and returns a logger",
			cfg: func() logging.Config {
				return logging.Config{
					Level:     "info",
					Output:    logging.OutputFile,
					FilePath:  filepath.Join(t.TempDir(), "nested", "app.log"),
					MaxSizeMB: 100,
				}
			}(),
			expectedLoggerNil: false,
			expectedError:     nil,
		},
		{
			name: "both mode returns a logger",
			cfg: func() logging.Config {
				return logging.Config{
					Level:     "info",
					Output:    logging.OutputBoth,
					FilePath:  filepath.Join(t.TempDir(), "app.log"),
					MaxSizeMB: 100,
				}
			}(),
			expectedLoggerNil: false,
			expectedError:     nil,
		},
		{
			name: "unknown output returns an error",
			cfg: logging.Config{
				Level:  "info",
				Output: "syslog",
			},
			expectedLoggerNil: true,
			expectedError:     logging.ErrLoggingUnknownOutputMode,
		},
		{
			name: "file mode without path returns an error",
			cfg: logging.Config{
				Level:  "info",
				Output: logging.OutputFile,
			},
			expectedLoggerNil: true,
			expectedError:     logging.ErrLoggingFilePathIsRequired,
		},
		{
			name: "file mode without positive size returns an error",
			cfg: logging.Config{
				Level:    "info",
				Output:   logging.OutputFile,
				FilePath: filepath.Join(t.TempDir(), "app.log"),
			},
			expectedLoggerNil: true,
			expectedError:     logging.ErrLoggingInvalidRotation,
		},
		{
			name: "negative rotation bound returns an error",
			cfg: logging.Config{
				Level:     "info",
				Output:    logging.OutputFile,
				FilePath:  filepath.Join(t.TempDir(), "app.log"),
				MaxSizeMB: -1,
			},
			expectedLoggerNil: true,
			expectedError:     logging.ErrLoggingInvalidRotation,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, err := logging.NewFromConfig(tc.cfg)
			if tc.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, tc.expectedError)
			}

			assert.Equal(t, tc.expectedLoggerNil, logger == nil)
		})
	}
}

// TestNewFromConfigWritesJSONFile verifies the essential file side effect
// permitted by the E09.3-T01 packet exception: NewFromConfig returns only
// (logger, error), so the persisted JSON line is observed from the file.
func TestNewFromConfigWritesJSONFile(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		cfg               logging.Config
		message           string
		expectedFragments []string
	}

	testCases := []testCase{
		{
			name: "file mode writes newline-delimited JSON",
			cfg: func() logging.Config {
				return logging.Config{
					Level:     "info",
					Output:    logging.OutputFile,
					FilePath:  filepath.Join(t.TempDir(), "nested", "app.log"),
					MaxSizeMB: 100,
				}
			}(),
			message: "file-visible",
			expectedFragments: []string{
				`"msg":"file-visible"`,
				`"component":"test"`,
			},
		},
		{
			name: "both mode also writes the same JSON stream to the file",
			cfg: func() logging.Config {
				return logging.Config{
					Level:     "info",
					Output:    logging.OutputBoth,
					FilePath:  filepath.Join(t.TempDir(), "both", "app.log"),
					MaxSizeMB: 100,
				}
			}(),
			message: "both-visible",
			expectedFragments: []string{
				`"msg":"both-visible"`,
				`"component":"test"`,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, err := logging.NewFromConfig(tc.cfg)
			require.NoError(t, err)
			require.NotNil(t, logger)

			logger.Info(context.Background(), tc.message, "component", "test")

			content, err := os.ReadFile(tc.cfg.FilePath)
			require.NoError(t, err)

			for _, fragment := range tc.expectedFragments {
				assert.Contains(t, string(content), fragment)
			}
		})
	}
}
