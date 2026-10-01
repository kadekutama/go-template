package migration

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// recordingLogger captures lines per severity for bridge assertions.
type recordingLogger struct {
	lines sync.Map // string -> *atomic.Pointer[[]string]
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{}
}

func (l *recordingLogger) record(level, msg string) {
	val, _ := l.lines.LoadOrStore(level, &atomic.Pointer[[]string]{})
	ptr := val.(*atomic.Pointer[[]string])
	for {
		old := ptr.Load()
		var next []string
		if old != nil {
			next = make([]string, len(*old)+1)
			copy(next, *old)
			next[len(*old)] = msg
		} else {
			next = []string{msg}
		}
		if ptr.CompareAndSwap(old, &next) {
			break
		}
	}
}

func (l *recordingLogger) getLines(level string) []string {
	val, ok := l.lines.Load(level)
	if !ok {
		return nil
	}
	ptr := val.(*atomic.Pointer[[]string])
	old := ptr.Load()
	if old == nil {
		return nil
	}
	out := make([]string, len(*old))
	copy(out, *old)
	return out
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

			lines := recorder.getLines(tc.expectedLevel)
			require.Len(t, lines, 1)
			assert.Equal(t, tc.expectedLine, lines[0])
		})
	}
}
