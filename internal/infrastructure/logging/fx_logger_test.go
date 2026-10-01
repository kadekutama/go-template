package logging_test

import (
	"context"
	"errors"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxevent"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// recordingLogger captures the level of every emitted line.
type recordingLogger struct {
	levels atomic.Pointer[[]string]
}

func (l *recordingLogger) emit(level string) {
	for {
		old := l.levels.Load()
		var next []string
		if old != nil {
			next = make([]string, len(*old)+1)
			copy(next, *old)
			next[len(*old)] = level
		} else {
			next = []string{level}
		}
		if l.levels.CompareAndSwap(old, &next) {
			break
		}
	}
}

func (l *recordingLogger) Trace(context.Context, string, ...any) { l.emit("trace") }
func (l *recordingLogger) Debug(context.Context, string, ...any) { l.emit("debug") }
func (l *recordingLogger) Info(context.Context, string, ...any)  { l.emit("info") }
func (l *recordingLogger) Warn(context.Context, string, ...any)  { l.emit("warn") }
func (l *recordingLogger) Error(context.Context, string, ...any) { l.emit("error") }
func (l *recordingLogger) Panic(_ context.Context, msg string, _ ...any) {
	panic(msg)
}
func (l *recordingLogger) Fatal(context.Context, string, ...any) {}
func (l *recordingLogger) With(...any) log.Logger                { return l }

func (l *recordingLogger) recorded() []string {
	old := l.levels.Load()
	if old == nil {
		return nil
	}
	out := make([]string, len(*old))
	copy(out, *old)

	return out
}

func TestNewFxLoggerValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		inner         log.Logger
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "nil inner fails fast",
			inner:         nil,
			expectedError: true,
		},
		{
			name:          "logger builds",
			inner:         &recordingLogger{},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			adapter, err := logging.NewFxLogger(tc.inner)
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, adapter)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, adapter)
		})
	}
}

func TestFxLoggerLevels(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		event         fxevent.Event
		expectedLevel string
	}

	testCases := []testCase{
		{
			name:          "started milestone at info",
			event:         &fxevent.Started{},
			expectedLevel: "info",
		},
		{
			name:          "stopping at info",
			event:         &fxevent.Stopping{Signal: syscall.SIGTERM},
			expectedLevel: "info",
		},
		{
			name:          "stopped clean at info",
			event:         &fxevent.Stopped{},
			expectedLevel: "info",
		},
		{
			name:          "provided wiring at debug",
			event:         &fxevent.Provided{ConstructorName: "di.ProvideLoggerWithConfig", OutputTypeNames: []string{"log.Logger"}},
			expectedLevel: "debug",
		},
		{
			name:          "invoking at debug",
			event:         &fxevent.Invoking{FunctionName: "main.main.func1"},
			expectedLevel: "debug",
		},
		{
			name:          "hook failure at error",
			event:         &fxevent.OnStartExecuted{FunctionName: "db.Open", Err: errors.New("dial refused")},
			expectedLevel: "error",
		},
		{
			name:          "invoke failure at error",
			event:         &fxevent.Invoked{FunctionName: "main.main.func1", Err: errors.New("missing type")},
			expectedLevel: "error",
		},
		{
			name:          "rollback at error",
			event:         &fxevent.RollingBack{StartErr: errors.New("start failed")},
			expectedLevel: "error",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingLogger{}
			adapter, err := logging.NewFxLogger(recorder)
			require.NoError(t, err)

			adapter.LogEvent(tc.event)

			recorded := recorder.recorded()
			require.Len(t, recorded, 1)
			assert.Equal(t, tc.expectedLevel, recorded[0])
		})
	}
}
