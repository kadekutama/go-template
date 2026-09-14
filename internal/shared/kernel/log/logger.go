// Package log is the backend-free logging port (E01-T04, SPEC §7.10).
//
// Domain and application layers import ONLY this package — never an adapter.
// Adapters (e.g. internal/infrastructure/logging) implement Logger and are
// swapped with a one-line fx binding change.
package log

import "context"

// Logger is the consumer-owned logging contract: all 7 zerolog severities
// (trace, debug, info, warn, error, panic, fatal).
//
// Panic and Fatal are control flow, not just severity:
//   - Panic emits the line, then panics with msg. The panic is an ordinary Go
//     panic: recover it with safe.Go, where the unit becomes failed/unknown
//     (never committed). Never call Panic for an expected error.
//   - Fatal emits the line, then terminates the process with exit code 1.
//     It is unrecoverable by design: call it only when the process cannot
//     serve (e.g. failed bootstrap). Tests stub the exit; production exits.
type Logger interface {
	Trace(ctx context.Context, msg string, args ...any)
	Debug(ctx context.Context, msg string, args ...any)
	Info(ctx context.Context, msg string, args ...any)
	Warn(ctx context.Context, msg string, args ...any)
	Error(ctx context.Context, msg string, args ...any)
	Panic(ctx context.Context, msg string, args ...any)
	Fatal(ctx context.Context, msg string, args ...any)
	// With returns a child Logger with args attached to every line.
	With(args ...any) Logger
}

// Correlation field keys propagated through contexts (SPEC §9.5).
const (
	FieldRequestID = "request_id"
	FieldTraceID   = "trace_id"
	FieldTenantID  = "tenant_id"
	FieldUserID    = "user_id"
)
