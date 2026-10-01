package di

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/shutdown"
)

// FxTimeouts resolves the fx OnStart/OnStop budgets from the process
// environment so operators tune them without code changes:
//
//	APP_FX_START_TIMEOUT Go duration, e.g. "45s" (start hooks, required)
//	APP_FX_STOP_TIMEOUT  Go duration, e.g. "60s" (drain hooks, required)
//
// Unset, malformed, or non-positive values fail fast: silently booting
// with the wrong budget would hide misconfigurations until an outage.
func FxTimeouts() (start, stop time.Duration, err error) {
	start, err = fxTimeoutEnv("APP_FX_START_TIMEOUT")
	if err != nil {
		return 0, 0, err
	}

	stop, err = fxTimeoutEnv("APP_FX_STOP_TIMEOUT")
	if err != nil {
		return 0, 0, err
	}

	return start, stop, nil
}

// fxTimeoutEnv parses one required duration key from the environment.
func fxTimeoutEnv(key string) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, fmt.Errorf("di: %s is required", key)
	}

	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("di: invalid %s %q: %w", key, raw, err)
	}

	if parsed <= 0 {
		return 0, fmt.Errorf("di: invalid %s %q: must be positive", key, raw)
	}

	return parsed, nil
}

// FxLogger routes fx framework events through the supplied kernel logger
// instead of fx.NopLogger. The logger comes from the binary entrypoint (which
// already failed fast on bad logging config), so this function performs no
// environment resolution of its own. A nil logger selects fx's console logger
// on stderr: framework events stay visible, and no panic path exists.
func FxLogger(logger log.Logger) fx.Option {
	return fx.WithLogger(func() fxevent.Logger {
		if logger == nil {
			return &fxevent.ConsoleLogger{W: os.Stderr}
		}

		adapter, err := logging.NewFxLogger(logger)
		if err != nil {
			return &fxevent.ConsoleLogger{W: os.Stderr}
		}

		return adapter
	})
}

// RunApp runs one binary end to end and returns its exit code: it logs the
// lifecycle (`initialising` → `running` → `shutting down` → `finished
// successfully`, every line carrying `binary` and `version`), resolves fx
// timeouts, builds and starts the graph, then either runs work to completion
// (batch CLIs: `work != nil`) or serves until SIGTERM/SIGINT (`work == nil`).
// `finished successfully` emits only when the drain completes cleanly, so a
// missing line pinpoints the slow/stuck phase in deploy logs. `version` is a
// release-stamped `-ldflags "-X main.version=..."` value (`dev` by default);
// mismatched versions across pods reveal stale deployments immediately.
func RunApp(logger log.Logger, binary, version string, work func(context.Context) error, opts ...fx.Option) int {
	ctx := context.Background()
	fields := []any{"binary", binary, "version", version}

	logger.Info(ctx, "app initialising", fields...)

	startTimeout, stopTimeout, err := FxTimeouts()
	if err != nil {
		logger.Error(ctx, "app timeout configuration invalid", append(fields, log.Err(err))...)
		return 1
	}

	app := fx.New(append(opts, FxLogger(logger), fx.StartTimeout(startTimeout), fx.StopTimeout(stopTimeout))...)

	if err := app.Start(ctx); err != nil {
		logger.Error(ctx, "app failed to start", append(fields, log.Err(err))...)
		return 1
	}

	logger.Info(ctx, "app running", fields...)

	failed := false

	if work != nil {
		if err := work(ctx); err != nil {
			logger.Error(ctx, "app work failed", append(fields, log.Err(err))...)
			failed = true
		}

		logger.Info(ctx, "app shutting down", fields...)

		if stopErr := app.Stop(ctx); stopErr != nil {
			logger.Error(ctx, "app shutdown failed", append(fields, log.Err(stopErr))...)
			failed = true
		}
	} else {
		drain := func(drainCtx context.Context) error {
			return app.Stop(drainCtx)
		}
		serve := func(serveCtx context.Context) error {
			<-serveCtx.Done()
			logger.Info(serveCtx, "app shutting down", fields...)

			return nil
		}

		if err := shutdown.Run(ctx, stopTimeout, serve, drain); err != nil {
			logger.Error(ctx, "app shutdown failed", append(fields, log.Err(err))...)
			failed = true
		}
	}

	if failed {
		return 1
	}

	logger.Info(ctx, "app finished successfully", fields...)

	return 0
}
