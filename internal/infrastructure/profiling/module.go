package profiling

import (
	"context"

	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// Module registers the profiler and its lifecycle hooks. The composition root
// supplies a profiling.Config (InfrastructureModule supplies the safe,
// default-disabled configuration until the binary maps the app config
// section).
func Module() fx.Option {
	return fx.Module("profiling",
		fx.Provide(func(cfg Config, logger log.Logger) ProfilerParams {
			return ProfilerParams{Config: cfg, Logger: logger}
		}),
		fx.Provide(NewProfiler),
		fx.Invoke(registerLifecycle),
	)
}

// registerLifecycle starts the profiler after dependencies are constructed and
// flushes/stops it in reverse order during shutdown. Disabled configs keep both
// hooks no-ops.
func registerLifecycle(lifecycle fx.Lifecycle, profiler *Profiler) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			return profiler.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			return profiler.Stop(ctx)
		},
	})
}
