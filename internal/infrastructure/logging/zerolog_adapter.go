// Package logging is the zerolog adapter behind the kernel log port
// (E01-T04, default backend per ADR-012 which supersedes ADR-010's slog
// default; the port is unchanged). Swap backends by adding a new adapter
// package implementing log.Logger and changing one fx.Provide line in
// internal/shared/di.
package logging

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"example.com/go-template/internal/shared/kernel/log"

	"github.com/rs/zerolog"
)

// Config tunes the adapter. Unknown levels fail open to info: logging must
// never break the caller (observability degrades, money logic does not).
type Config struct {
	Level string
}

// configureShape pins the process-wide zerolog JSON shape once (the sanctioned
// exception to no-global-state, mirroring the OTel globals in E01-T05):
// RFC3339Nano UTC timestamps, conventional field names, and the "msg" message
// key our envelope contract asserts. E11 log parsers may rely on this shape.
var configureShape = sync.OnceFunc(func() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	zerolog.TimestampFieldName = "time"
	zerolog.LevelFieldName = "level"
	zerolog.MessageFieldName = "msg"
})

func parseLevel(raw string) zerolog.Level {
	level, err := zerolog.ParseLevel(raw)
	if err != nil {
		return zerolog.InfoLevel
	}
	return level
}

// zerologLogger implements log.Logger over a zerolog.Logger value.
type zerologLogger struct {
	inner zerolog.Logger
}

// New builds a JSON logger writing to w. Call-site note: zerolog performs no
// sampling or hooks here; E15 adds those at the pipeline.
func New(w io.Writer, cfg Config) log.Logger {
	configureShape()
	return &zerologLogger{inner: zerolog.New(w).Level(parseLevel(cfg.Level))}
}

// fields merges context correlation fields (SPEC §9.5) with call args into one
// map; call args win key collisions.
func fields(ctx context.Context, args []any) map[string]any {
	merged := make(map[string]any, len(args)/2+4)
	for key, val := range log.FromContext(ctx) {
		merged[key] = val
	}
	for i := 0; i+1 < len(args); i += 2 {
		key, ok := args[i].(string)
		if !ok {
			key = fmt.Sprintf("%v", args[i])
		}
		merged[key] = args[i+1]
	}
	if len(args)%2 != 0 {
		merged["!EXTRA_ARG"] = args[len(args)-1]
	}
	return merged
}

func (l *zerologLogger) emit(level zerolog.Level, ctx context.Context, msg string, args []any) {
	event := l.inner.WithLevel(level).Fields(fields(ctx, args)).Timestamp()
	// WithLevel returns nil for disabled levels; Msg on nil is a safe no-op.
	event.Msg(msg)
}

// osExit terminates the process for Fatal. It is a variable (not a direct
// os.Exit call) so tests can stub it; production always exits.
var osExit = os.Exit

func (l *zerologLogger) Trace(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.TraceLevel, ctx, msg, args)
}

func (l *zerologLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.DebugLevel, ctx, msg, args)
}

func (l *zerologLogger) Info(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.InfoLevel, ctx, msg, args)
}

func (l *zerologLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.WarnLevel, ctx, msg, args)
}

func (l *zerologLogger) Error(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.ErrorLevel, ctx, msg, args)
}

// Panic emits the line, then panics with msg (recoverable via safe.Go).
func (l *zerologLogger) Panic(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.PanicLevel, ctx, msg, args)
	panic(msg)
}

// Fatal emits the line, then terminates the process with code 1.
// Unrecoverable by design: bootstrap failures only.
func (l *zerologLogger) Fatal(ctx context.Context, msg string, args ...any) {
	l.emit(zerolog.FatalLevel, ctx, msg, args)
	osExit(1)
}

func (l *zerologLogger) With(args ...any) log.Logger {
	return &zerologLogger{inner: l.inner.With().Fields(fields(context.Background(), args)).Logger()}
}

// discardLogger drops every line; unit tests use it instead of I/O.
type discardLogger struct{}

func (discardLogger) Trace(context.Context, string, ...any) {}
func (discardLogger) Debug(context.Context, string, ...any) {}
func (discardLogger) Info(context.Context, string, ...any)  {}
func (discardLogger) Warn(context.Context, string, ...any)  {}
func (discardLogger) Error(context.Context, string, ...any) {}
func (discardLogger) Panic(_ context.Context, msg string, _ ...any) {
	panic(msg)
}
func (discardLogger) Fatal(_ context.Context, _ string, _ ...any) {
	osExit(1)
}
func (discardLogger) With(...any) log.Logger { return discardLogger{} }

// Discard returns a Logger that drops every line.
func Discard() log.Logger { return discardLogger{} }
