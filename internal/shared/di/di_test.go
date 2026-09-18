package di

import (
	"bytes"
	"context"
	"errors"
	"sync"
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
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *testLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *testLogBuffer) lines() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
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
			name:          "unset environment keeps defaults",
			startEnv:      "",
			stopEnv:       "",
			expectedStart: 30 * time.Second,
			expectedStop:  30 * time.Second,
			expectError:   false,
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
			name:        "malformed start fails fast",
			startEnv:    "soon",
			stopEnv:     "",
			expectError: true,
		},
		{
			name:        "non-positive stop fails fast",
			startEnv:    "",
			stopEnv:     "0s",
			expectError: true,
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
		assert.NotNil(t, FxLogger())
		assert.NoError(t, fx.ValidateApp(FxLogger()))
	})
}

func TestProvidersConstructInstances(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		validate func() bool
	}

	testCases := []testCase{
		{
			name: "logger provider returns non-nil",
			validate: func() bool {
				return ProvideLogger() != nil
			},
		},
		{
			name: "clock provider returns valid clock",
			validate: func() bool {
				clk := ProvideClock()
				return clk != nil && !clk.Now().IsZero()
			},
		},
		{
			name: "id generator provider returns valid generator",
			validate: func() bool {
				gen := ProvideIDGenerator()
				return gen != nil && gen.NewID() != ""
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.True(t, tc.validate())
		})
	}
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

		var output testLogBuffer
		logger := logging.New(&output, logging.Config{Level: "trace"})

		code := RunApp(logger, "test-binary", "v0", nil)
		assert.Equal(t, 1, code)
		assert.Contains(t, output.lines(), "app timeout configuration invalid")
	})
}
