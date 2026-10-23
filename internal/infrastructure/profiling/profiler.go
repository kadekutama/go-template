package profiling

import (
	"context"
	"fmt"
	"maps"
	"runtime"

	otelpyroscope "github.com/grafana/otel-profiling-go"
	"github.com/grafana/pyroscope-go"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// Profiler owns one Pyroscope SDK handle and its lifecycle. The SDK handle is
// created only by Start and is never exposed, keeping Pyroscope types inside
// this adapter. fx serializes lifecycle hooks, so Start and Stop are not
// intended for concurrent calls.
type Profiler struct {
	cfg      Config
	logger   log.Logger
	instance *pyroscope.Profiler
}

// NewProfiler validates the configuration and nil logger dependency. It does
// not start profiling: Start is the single place that opens connections and
// mutates process-global runtime sampling.
func NewProfiler(params ProfilerParams) (*Profiler, error) {
	if params.Logger == nil {
		return nil, ErrNilLogger
	}

	if err := params.Config.Validate(); err != nil {
		return nil, err
	}

	return &Profiler{
		cfg:    params.Config.clone(),
		logger: params.Logger,
	}, nil
}

// Start starts continuous profiling when enabled. Disabled configurations are
// a zero-allocation no-op: no network and no runtime-rate mutation. Sequential
// repeated Start calls are idempotent.
func (p *Profiler) Start(ctx context.Context) error {
	if p == nil {
		return ErrNotInitialized
	}

	if !p.cfg.Enabled {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("profiling: start: %w", err)
	}

	if p.instance != nil {
		return nil
	}

	runtime.SetMutexProfileFraction(p.cfg.MutexProfileFraction)
	runtime.SetBlockProfileRate(p.cfg.BlockProfileRate)

	instance, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: p.cfg.ApplicationName,
		ServerAddress:   p.cfg.ServerAddress,
		UploadRate:      p.cfg.UploadRate,
		Logger:          pyroscopeLogger{logger: p.logger},
		ProfileTypes:    profileTypes(),
		DisableGCRuns:   p.cfg.DisableGCRuns,
		Tags:            profilerTags(p.cfg),
	})
	if err != nil {
		runtime.SetMutexProfileFraction(0)
		runtime.SetBlockProfileRate(0)

		return fmt.Errorf("%w: %w", ErrStartFailed, err)
	}

	p.instance = instance
	p.logger.Info(ctx, "profiling.started",
		"application", p.cfg.ApplicationName,
		"server", p.cfg.ServerAddress,
	)

	return nil
}

// Stop flushes buffered profiles, stops the SDK handle, and resets the
// process-global mutex/block rates. Sequential repeated Stop calls are
// idempotent; disabled configurations remain a zero-allocation no-op and never
// touch runtime state.
func (p *Profiler) Stop(ctx context.Context) error {
	if p == nil {
		return ErrNotInitialized
	}

	if !p.cfg.Enabled {
		return nil
	}

	if p.instance == nil {
		return nil
	}

	instance := p.instance
	p.instance = nil

	instance.Flush(true)

	err := instance.Stop()

	runtime.SetMutexProfileFraction(0)
	runtime.SetBlockProfileRate(0)

	if err != nil {
		return fmt.Errorf("%w: %w", ErrStopFailed, err)
	}

	p.logger.Info(ctx, "profiling.stopped")

	return nil
}

// WrapTracerProvider decorates an OTel provider so sampled root spans receive
// the pyroscope.profile.id attribute and pprof span labels (ADR-027). A nil
// provider is returned unchanged; callers only wrap when profiling is enabled.
//
//nolint:ireturn // The packet contract returns the OTel TracerProvider interface.
func WrapTracerProvider(tp oteltrace.TracerProvider) oteltrace.TracerProvider {
	if tp == nil {
		return nil
	}

	return otelpyroscope.NewTracerProvider(tp)
}

// profileTypes is the ADR-027 profile set. CPU is required for the trace to
// profile correlation; heap, goroutine, mutex, and block profiles cover the
// remaining runtime pressure signals.
func profileTypes() []pyroscope.ProfileType {
	return []pyroscope.ProfileType{
		pyroscope.ProfileCPU,
		pyroscope.ProfileAllocObjects,
		pyroscope.ProfileAllocSpace,
		pyroscope.ProfileInuseObjects,
		pyroscope.ProfileInuseSpace,
		pyroscope.ProfileGoroutines,
		pyroscope.ProfileMutexCount,
		pyroscope.ProfileMutexDuration,
		pyroscope.ProfileBlockCount,
		pyroscope.ProfileBlockDuration,
	}
}

// profilerTags merges configured static tags with the authoritative service
// and environment labels. Service/env cannot be overridden by operator tags.
func profilerTags(cfg Config) map[string]string {
	tags := maps.Clone(cfg.Tags)
	if tags == nil {
		tags = map[string]string{}
	}

	if cfg.ApplicationName != "" {
		tags["service"] = cfg.ApplicationName
	}

	if cfg.Environment != "" {
		tags["env"] = cfg.Environment
	}

	return tags
}

// pyroscopeLogger adapts the kernel logging port to the SDK's printf-style
// logger. The SDK calls these methods from background goroutines.
type pyroscopeLogger struct {
	logger log.Logger
}

// Infof implements pyroscope.Logger.
func (l pyroscopeLogger) Infof(format string, args ...any) {
	l.logger.Info(context.Background(), fmt.Sprintf(format, args...))
}

// Debugf implements pyroscope.Logger.
func (l pyroscopeLogger) Debugf(format string, args ...any) {
	l.logger.Debug(context.Background(), fmt.Sprintf(format, args...))
}

// Errorf implements pyroscope.Logger.
func (l pyroscopeLogger) Errorf(format string, args ...any) {
	l.logger.Error(context.Background(), fmt.Sprintf(format, args...))
}
