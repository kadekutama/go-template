// FxLogger adapts the kernel log port to fxevent.Logger so fx framework
// events flow through the configured backend (zerolog per ADR-012) instead
// of fx.NopLogger (E01-T04-R06). Lifecycle milestones log at Info, routine
// wiring at Debug, and every hook/supply/invoke failure at Error.
package logging

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/fx/fxevent"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// FxLogger is an fxevent.Logger backed by the kernel log port.
type FxLogger struct {
	inner log.Logger
}

// NewFxLogger builds an fxevent.Logger over inner. A nil inner is a
// programmer error and fails fast: silently dropping framework events would
// hide startup failures.
func NewFxLogger(inner log.Logger) (*FxLogger, error) {
	if inner == nil {
		return nil, fmt.Errorf("logging: fx logger requires a non-nil kernel logger")
	}

	return &FxLogger{inner: inner}, nil
}

// fail emits msg at Error when err is set and reports whether it did.
func (l *FxLogger) fail(ctx context.Context, msg string, err error, fields ...any) bool {
	if err == nil {
		return false
	}

	l.inner.Error(ctx, msg, append(fields, log.Err(err))...)
	return true
}

// LogEvent implements fxevent.Logger by delegating to per-family mappers.
func (l *FxLogger) LogEvent(event fxevent.Event) {
	if l.logHookEvent(event) {
		return
	}

	if l.logWireEvent(event) {
		return
	}

	if l.logInvokeEvent(event) {
		return
	}

	l.logLifecycleEvent(event)
}

// logHookEvent maps hook execution events. It reports whether it handled event.
func (l *FxLogger) logHookEvent(event fxevent.Event) bool {
	ctx := context.Background()

	switch e := event.(type) {
	case *fxevent.OnStartExecuting:
		l.inner.Debug(ctx, "fx hook starting", "hook", "OnStart", "function", e.FunctionName, "caller", e.CallerName)
	case *fxevent.OnStartExecuted:
		if !l.fail(ctx, "fx hook failed", e.Err, "hook", "OnStart", "function", e.FunctionName, "caller", e.CallerName) {
			l.inner.Debug(ctx, "fx hook ran", "hook", "OnStart", "function", e.FunctionName, "caller", e.CallerName)
		}
	case *fxevent.OnStopExecuting:
		l.inner.Debug(ctx, "fx hook starting", "hook", "OnStop", "function", e.FunctionName, "caller", e.CallerName)
	case *fxevent.OnStopExecuted:
		if !l.fail(ctx, "fx hook failed", e.Err, "hook", "OnStop", "function", e.FunctionName, "caller", e.CallerName) {
			l.inner.Debug(ctx, "fx hook ran", "hook", "OnStop", "function", e.FunctionName, "caller", e.CallerName)
		}
	default:
		return false
	}

	return true
}

// logWireEvent maps dependency-wiring events. It reports whether it handled event.
func (l *FxLogger) logWireEvent(event fxevent.Event) bool {
	ctx := context.Background()

	switch e := event.(type) {
	case *fxevent.Supplied:
		if !l.fail(ctx, "fx supply failed", e.Err, "type", e.TypeName) {
			l.inner.Debug(ctx, "fx supplied", "type", e.TypeName, "module", e.ModuleName)
		}
	case *fxevent.Provided:
		if !l.fail(ctx, "fx provide failed", e.Err, "constructor", e.ConstructorName) {
			l.inner.Debug(ctx, "fx provided", "types", strings.Join(e.OutputTypeNames, ","), "constructor", e.ConstructorName)
		}
	case *fxevent.Replaced:
		if !l.fail(ctx, "fx replace failed", e.Err) {
			l.inner.Debug(ctx, "fx replaced", "types", strings.Join(e.OutputTypeNames, ","))
		}
	case *fxevent.Decorated:
		if !l.fail(ctx, "fx decorate failed", e.Err) {
			l.inner.Debug(ctx, "fx decorated", "types", strings.Join(e.OutputTypeNames, ","))
		}
	case *fxevent.BeforeRun:
		l.inner.Debug(ctx, "fx before run", "kind", e.Kind, "name", e.Name)
	case *fxevent.Run:
		if !l.fail(ctx, "fx run failed", e.Err, "kind", e.Kind, "name", e.Name) {
			l.inner.Debug(ctx, "fx ran", "kind", e.Kind, "name", e.Name)
		}
	default:
		return false
	}

	return true
}

// logInvokeEvent maps invocation events. It reports whether it handled event.
func (l *FxLogger) logInvokeEvent(event fxevent.Event) bool {
	ctx := context.Background()

	switch e := event.(type) {
	case *fxevent.Invoking:
		l.inner.Debug(ctx, "fx invoking", "function", e.FunctionName)
	case *fxevent.Invoked:
		l.fail(ctx, "fx invoke failed", e.Err, "function", e.FunctionName)
	default:
		return false
	}

	return true
}

// logLifecycleEvent maps process-lifecycle events and anything unrecognized.
func (l *FxLogger) logLifecycleEvent(event fxevent.Event) {
	ctx := context.Background()

	switch e := event.(type) {
	case *fxevent.Stopping:
		l.inner.Info(ctx, "fx stopping", "signal", strings.ToUpper(e.Signal.String()))
	case *fxevent.Stopped:
		if !l.fail(ctx, "fx stop failed", e.Err) {
			l.inner.Info(ctx, "fx stopped")
		}
	case *fxevent.RollingBack:
		l.inner.Error(ctx, "fx start failed, rolling back", log.Err(e.StartErr))
	case *fxevent.RolledBack:
		if !l.fail(ctx, "fx rollback failed", e.Err) {
			l.inner.Debug(ctx, "fx rolled back")
		}
	case *fxevent.Started:
		if !l.fail(ctx, "fx start failed", e.Err) {
			l.inner.Info(ctx, "fx started")
		}
	case *fxevent.LoggerInitialized:
		if !l.fail(ctx, "fx logger init failed", e.Err) {
			l.inner.Debug(ctx, "fx logger initialized", "constructor", e.ConstructorName)
		}
	default:
		l.inner.Debug(ctx, "fx event", "type", fmt.Sprintf("%T", event))
	}
}
