package di

import (
	"context"
	"errors"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
)

// errTestWorkFailed marks the failing-work scenario.
var errTestWorkFailed = errors.New("test work failed")

// testLogBuffer captures logger output for lifecycle assertions.
type testLogBuffer struct {
	buf atomic.Pointer[[]byte]
}

func (b *testLogBuffer) Write(p []byte) (int, error) {
	for {
		old := b.buf.Load()
		var next []byte
		if old != nil {
			next = make([]byte, len(*old)+len(p))
			copy(next, *old)
			copy(next[len(*old):], p)
		} else {
			next = make([]byte, len(p))
			copy(next, p)
		}
		if b.buf.CompareAndSwap(old, &next) {
			break
		}
	}
	return len(p), nil
}

func (b *testLogBuffer) lines() string {
	old := b.buf.Load()
	if old == nil {
		return ""
	}
	return string(*old)
}

// TestGraphValidates proves the composed fx graph has no dependency cycles.
// Every binary module must appear here; a cycle fails the test before any
// binary is built.
func TestGraphValidates(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		DomainModule(),
		ApplicationModule(),
		InfrastructureModule(),
		RestModule(),
		GrpcModule(),
		GraphQLModule(),
		CronModule(),
		ConsumerModule(),
		fx.NopLogger,
	)
	assert.NoError(t, err)
}

func TestFxLifecycle(t *testing.T) {
	t.Setenv("APP_LOG_LEVEL", "info")

	type testCase struct {
		name          string
		startEnv      string
		stopEnv       string
		expectedStart time.Duration
		expectedStop  time.Duration
		expectError   bool
	}

	testCases := []testCase{
		{
			name:          "unset environment fails fast",
			startEnv:      "",
			stopEnv:       "",
			expectedStart: 0,
			expectedStop:  0,
			expectError:   true,
		},
		{
			name:          "unset start fails fast",
			startEnv:      "",
			stopEnv:       "30s",
			expectedStart: 0,
			expectedStop:  0,
			expectError:   true,
		},
		{
			name:          "unset stop fails fast",
			startEnv:      "30s",
			stopEnv:       "",
			expectedStart: 0,
			expectedStop:  0,
			expectError:   true,
		},
		{
			name:          "valid durations apply",
			startEnv:      "45s",
			stopEnv:       "1m30s",
			expectedStart: 45 * time.Second,
			expectedStop:  90 * time.Second,
			expectError:   false,
		},
		{
			name:          "malformed start fails fast",
			startEnv:      "soon",
			stopEnv:       "30s",
			expectedStart: 0,
			expectedStop:  0,
			expectError:   true,
		},
		{
			name:          "non-positive stop fails fast",
			startEnv:      "30s",
			stopEnv:       "0s",
			expectedStart: 0,
			expectedStop:  0,
			expectError:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_FX_START_TIMEOUT", tc.startEnv)
			t.Setenv("APP_FX_STOP_TIMEOUT", tc.stopEnv)

			start, stop, err := FxTimeouts()
			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expectedStart, start)
			assert.Equal(t, tc.expectedStop, stop)
		})
	}

	t.Run("logger option validates in a graph", func(t *testing.T) {
		logger, err := ProvideLoggerWithConfig(logging.Config{Level: "info"})
		require.NoError(t, err)
		assert.NotNil(t, FxLogger(logger))
		assert.NoError(t, fx.ValidateApp(FxLogger(logger)))
	})
}

func TestProvidersConstructInstances(t *testing.T) {
	type testCase struct {
		name     string
		validate func(t *testing.T) bool
	}

	testCases := []testCase{
		{
			name: "logger provider with config returns non-nil",
			validate: func(t *testing.T) bool {
				t.Helper()
				logger, err := ProvideLoggerWithConfig(logging.Config{Level: "info"})
				return err == nil && logger != nil
			},
		},
		{
			name: "logger config from env resolves",
			validate: func(t *testing.T) bool {
				t.Helper()
				t.Setenv("APP_LOG_LEVEL", "info")
				cfg, err := ProvideLoggingConfig()
				if err != nil {
					return false
				}
				logger, err := ProvideLoggerWithConfig(cfg)
				return err == nil && logger != nil
			},
		},
		{
			name: "clock provider returns valid clock",
			validate: func(t *testing.T) bool {
				t.Helper()
				clk := ProvideClock()
				return clk != nil && !clk.Now().IsZero()
			},
		},
		{
			name: "id generator provider returns valid generator",
			validate: func(t *testing.T) bool {
				t.Helper()
				gen := ProvideIDGenerator()
				return gen != nil && gen.NewID() != ""
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.validate(t))
		})
	}

	t.Run("logging config errors when unconfigured", func(t *testing.T) {
		t.Setenv("APP_LOG_LEVEL", "")
		t.Setenv("APP_OBSERVABILITY__LOG_LEVEL", "")
		_, err := ProvideLoggingConfig()
		require.Error(t, err)
	})
}

func TestRunAppLifecycle(t *testing.T) {
	type testCase struct {
		name              string
		work              func(ctx context.Context) error
		sendSignal        bool
		expectedCode      int
		expectedMessages  []string
		unexpectedMessage string
	}

	testCases := []testCase{
		{
			name: "batch work success logs full lifecycle",
			work: func(context.Context) error {
				return nil
			},
			sendSignal:        false,
			expectedCode:      0,
			expectedMessages:  []string{"app initialising", "app running", "app shutting down", "app finished successfully"},
			unexpectedMessage: "",
		},
		{
			name: "batch work failure exits without finished",
			work: func(context.Context) error {
				return errTestWorkFailed
			},
			sendSignal:        false,
			expectedCode:      1,
			expectedMessages:  []string{"app initialising", "app running", "app work failed", "app shutting down"},
			unexpectedMessage: "app finished successfully",
		},
		{
			name:              "signal-driven server stops cleanly",
			work:              nil,
			sendSignal:        true,
			expectedCode:      0,
			expectedMessages:  []string{"app initialising", "app running", "app shutting down", "app finished successfully"},
			unexpectedMessage: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_FX_START_TIMEOUT", "30s")
			t.Setenv("APP_FX_STOP_TIMEOUT", "30s")
			t.Setenv("APP_LOG_LEVEL", "info")

			var output testLogBuffer
			logger := logging.New(&output, logging.Config{Level: "trace"})

			if tc.sendSignal {
				go func() {
					time.Sleep(300 * time.Millisecond)
					_ = syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
				}()
			}

			code := RunApp(logger, "test-binary", "v9.9.9-test", tc.work)
			assert.Equal(t, tc.expectedCode, code)

			lines := output.lines()
			for _, want := range tc.expectedMessages {
				assert.Contains(t, lines, want, "lifecycle line %q missing", want)
			}

			if tc.unexpectedMessage != "" {
				assert.NotContains(t, lines, tc.unexpectedMessage)
			}

			assert.Contains(t, lines, `"binary":"test-binary"`)
			assert.Contains(t, lines, `"version":"v9.9.9-test"`)
		})
	}

	t.Run("malformed timeout env fails fast", func(t *testing.T) {
		t.Setenv("APP_FX_START_TIMEOUT", "soon")
		t.Setenv("APP_FX_STOP_TIMEOUT", "30s")
		t.Setenv("APP_LOG_LEVEL", "info")

		var output testLogBuffer
		logger := logging.New(&output, logging.Config{Level: "trace"})

		code := RunApp(logger, "test-binary", "v0", nil)
		assert.Equal(t, 1, code)
		assert.Contains(t, output.lines(), "app timeout configuration invalid")
	})

	t.Run("unset timeout env fails fast", func(t *testing.T) {
		t.Setenv("APP_FX_START_TIMEOUT", "")
		t.Setenv("APP_FX_STOP_TIMEOUT", "")
		t.Setenv("APP_LOG_LEVEL", "info")

		var output testLogBuffer
		logger := logging.New(&output, logging.Config{Level: "trace"})

		code := RunApp(logger, "test-binary", "v0", nil)
		assert.Equal(t, 1, code)
		assert.Contains(t, output.lines(), "app timeout configuration invalid")
	})
}
