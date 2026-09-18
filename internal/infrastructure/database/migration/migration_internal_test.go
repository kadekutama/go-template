package migration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// recordingLogger captures lines per severity for bridge assertions.
type recordingLogger struct {
	mu    sync.Mutex
	lines map[string][]string
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{lines: make(map[string][]string)}
}

func (l *recordingLogger) record(level, msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines[level] = append(l.lines[level], msg)
}

func (l *recordingLogger) Trace(context.Context, string, ...any) {}
func (l *recordingLogger) Debug(context.Context, string, ...any) {}
func (l *recordingLogger) Info(_ context.Context, msg string, _ ...any) {
	l.record("info", msg)
}
func (l *recordingLogger) Warn(context.Context, string, ...any) {}
func (l *recordingLogger) Error(_ context.Context, msg string, _ ...any) {
	l.record("error", msg)
}
func (l *recordingLogger) Panic(_ context.Context, msg string, _ ...any) { panic(msg) }
func (l *recordingLogger) Fatal(context.Context, string, ...any)         {}
func (l *recordingLogger) With(...any) log.Logger {
	return l
}

func TestGooseLoggerBridge(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		format        string
		args          []any
		isFatal       bool
		expectedLevel string
		expectedLine  string
	}

	testCases := []testCase{
		{
			name:          "printf maps to info",
			format:        "OK %s (%s)",
			args:          []any{"000001_ledger_core.sql", "1ms"},
			isFatal:       false,
			expectedLevel: "info",
			expectedLine:  "OK 000001_ledger_core.sql (1ms)",
		},
		{
			name:          "fatalf maps to error without exiting",
			format:        "migration %d failed",
			args:          []any{7},
			isFatal:       true,
			expectedLevel: "error",
			expectedLine:  "migration 7 failed",
		},
		{
			name:          "nil logger resolves to silence",
			format:        "ignored",
			args:          nil,
			isFatal:       false,
			expectedLevel: "",
			expectedLine:  "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectedLevel == "" {
				resolved := gooseLog(nil)
				assert.NotNil(t, resolved)
				resolved.Printf(tc.format, tc.args...)
				resolved.Fatalf(tc.format, tc.args...)

				return
			}

			recorder := newRecordingLogger()
			bridge := gooseLog(recorder)

			if tc.isFatal {
				bridge.Fatalf(tc.format, tc.args...)
			} else {
				bridge.Printf(tc.format, tc.args...)
			}

			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			require.Len(t, recorder.lines[tc.expectedLevel], 1)
			assert.Equal(t, tc.expectedLine, recorder.lines[tc.expectedLevel][0])
		})
	}
}

func TestResolveTimeout(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		timeout         time.Duration
		expectedTimeout time.Duration
	}

	testCases := []testCase{
		{
			name:            "zero selects default",
			timeout:         0,
			expectedTimeout: BootstrapDefaultTimeout,
		},
		{
			name:            "negative selects default",
			timeout:         -time.Second,
			expectedTimeout: BootstrapDefaultTimeout,
		},
		{
			name:            "explicit timeout kept",
			timeout:         5 * time.Second,
			expectedTimeout: 5 * time.Second,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedTimeout, resolveTimeout(tc.timeout))
		})
	}
}
