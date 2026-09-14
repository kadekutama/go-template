package di

import (
	"os"

	"example.com/go-template/internal/infrastructure/logging"
	"example.com/go-template/internal/shared/kernel"
	"example.com/go-template/internal/shared/kernel/log"

	"go.uber.org/fx"
)

// InfrastructureModule exposes infrastructure adapters (logging, clock, id generator;
// E01-T05 tracing is wired in E11).
func InfrastructureModule() fx.Option {
	return fx.Module("infrastructure",
		fx.Provide(
			ProvideLogger,
			ProvideClock,
			ProvideIDGenerator,
		),
	)
}

// ProvideLogger is the single fx binding for log.Logger: swapping the logging
// backend means changing this one call site (E01-T04-R04).
func ProvideLogger() log.Logger {
	return logging.New(os.Stdout, logging.DefaultConfig)
}

// ProvideClock is the fx binding for kernel.Clock.
func ProvideClock() kernel.Clock {
	return kernel.SystemClock{}
}

// ProvideIDGenerator is the fx binding for kernel.IDGenerator (UUIDv7 per ADR-012).
func ProvideIDGenerator() kernel.IDGenerator {
	return kernel.NewUUIDGenerator()
}
