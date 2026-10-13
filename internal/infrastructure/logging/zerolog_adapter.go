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

	"github.com/kadekutama/go-template/internal/shared/kernel/log"

	"github.com/rs/zerolog"
)

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
	// exit terminates the process for Fatal; injected for tests so parallel
	// tests never mutate process-global state.
	exit func(int)
}

// New builds a JSON logger writing to w. Call-site note: zerolog performs no
// sampling or hooks here; E15 adds those at the pipeline.
func New(w io.Writer, cfg Config) log.Logger {
	configureShape()

	return &zerologLogger{inner: zerolog.New(w).Level(parseLevel(cfg.Level)), exit: os.Exit}
}

// fields merges context correlation fields (SPEC §9.5) with call args into one
// map; call args win key collisions. It recognizes log.Field, key-value pairs,
// and captures any trailing odd argument under !EXTRA_ARG.
func fields(ctx context.Context, args []any) map[string]any {
	merged := make(map[string]any, len(args)+mergedFieldHeadroom)
	for key, val := range log.FromContext(ctx) {
		merged[key] = val
	}

	for i := 0; i < len(args); {
		if f, ok := args[i].(log.Field); ok {
			if f.Key == log.FieldError && f.Value == nil {
				i++

				continue
			}

			merged[f.Key] = f.Value
			i++

			continue
		}

		if i+1 < len(args) {
			key, ok := args[i].(string)
			if !ok {
				key = fmt.Sprintf("%v", args[i])
			}

			merged[key] = args[i+1]
			i += 2

			continue
		}

		merged["!EXTRA_ARG"] = args[i]
		i++
	}

	return merged
}

func (l *zerologLogger) Trace(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.TraceLevel, msg, args)
}

func (l *zerologLogger) Debug(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.DebugLevel, msg, args)
}

func (l *zerologLogger) Info(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.InfoLevel, msg, args)
}

func (l *zerologLogger) Warn(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.WarnLevel, msg, args)
}

func (l *zerologLogger) Error(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.ErrorLevel, msg, args)
}

// Panic emits the line, then panics with msg (recoverable via safe.Go).
func (l *zerologLogger) Panic(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.PanicLevel, msg, args)
	panic(msg)
}

// Fatal emits the line, then terminates the process with code 1.
// Unrecoverable by design: bootstrap failures only.
func (l *zerologLogger) Fatal(ctx context.Context, msg string, args ...any) {
	l.emit(ctx, zerolog.FatalLevel, msg, args)
	l.exit(1)
}

func (l *zerologLogger) With(args ...any) log.Logger {
	return &zerologLogger{inner: l.inner.With().Fields(fields(context.Background(), args)).Logger()}
}
func (l *zerologLogger) emit(ctx context.Context, level zerolog.Level, msg string, args []any) {
	event := l.inner.WithLevel(level).Fields(fields(ctx, args)).Timestamp()
	// WithLevel returns nil for disabled levels; Msg on nil is a safe no-op.
	event.Msg(msg)
}

// exit terminates the process for Fatal. It is a per-logger field (not a
// process-global variable) so parallel tests can stub it per instance;
// production always uses os.Exit via New.
