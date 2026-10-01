package di

import (
	"errors"
	"os"
	"strings"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"

	"go.uber.org/fx"
)

// InfrastructureModule exposes infrastructure adapters (logging, clock, id generator;
// E01-T05 tracing is wired in E11).
func InfrastructureModule() fx.Option {
	return fx.Module("infrastructure",
		fx.Provide(
			ProvideLoggingConfig,
			ProvideLoggerWithConfig,
			ProvideClock,
			ProvideIDGenerator,
		),
	)
}

// ProvideLoggingConfig resolves the logging configuration from the environment,
// requiring an explicit log level so it never silently defaults.
func ProvideLoggingConfig() (logging.Config, error) {
	raw := strings.TrimSpace(os.Getenv("APP_LOG_LEVEL"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("APP_OBSERVABILITY__LOG_LEVEL"))
	}
	if raw == "" {
		return logging.Config{}, errors.New("logging: log level is required (set APP_LOG_LEVEL or APP_OBSERVABILITY__LOG_LEVEL)")
	}

	cfg := logging.Config{Level: raw}
	if err := cfg.Validate(); err != nil {
		return logging.Config{}, err
	}

	return cfg, nil
}

// ProvideLoggerWithConfig is the fx binding for log.Logger: swapping the logging
// backend means changing this one call site (E01-T04-R04). Invalid configs
// fail as errors (fx aborts the graph with the message); callers must not
// pass unvalidated configs — resolve via ProvideLoggingConfig first.
func ProvideLoggerWithConfig(cfg logging.Config) (log.Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return logging.New(os.Stdout, cfg), nil
}

// ProvideClock is the fx binding for kernel.Clock.
func ProvideClock() kernel.Clock {
	return kernel.SystemClock{}
}

// ProvideIDGenerator is the fx binding for kernel.IDGenerator (UUIDv7 per ADR-012).
func ProvideIDGenerator() kernel.IDGenerator {
	return kernel.NewUUIDGenerator()
}
