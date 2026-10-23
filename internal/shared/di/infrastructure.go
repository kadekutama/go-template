package di

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/infrastructure/profiling"
	"github.com/kadekutama/go-template/internal/shared/kernel"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"

	"go.uber.org/fx"
)

const (
	defaultLogMaxSizeMB  = 100
	defaultLogMaxBackups = 7
	defaultLogMaxAgeDays = 30
	defaultLogCompress   = true
)

// InfrastructureModule exposes infrastructure adapters (logging, clock, id
// generator, continuous profiling). A zero-value profiling.Config is disabled,
// so the module is safe until a composition root maps the validated
// observability.profiling section. Callers may pass one config to override it.
func InfrastructureModule(profilingConfigs ...profiling.Config) fx.Option {
	var profilingConfig profiling.Config
	if len(profilingConfigs) > 0 {
		profilingConfig = profilingConfigs[0]
	}

	return fx.Module("infrastructure",
		fx.Provide(
			ProvideLoggingConfig,
			ProvideLoggerWithConfig,
			ProvideClock,
			ProvideIDGenerator,
		),
		fx.Supply(profilingConfig),
		profiling.Module(),
	)
}

// ProvideLoggingConfig resolves the logging configuration from the environment,
// requiring an explicit log level so it never silently defaults. Local runs
// default to stdout; explicit staging/production environments default to
// rotating file output. An explicit output mode always wins.
func ProvideLoggingConfig() (logging.Config, error) {
	raw := envFirst("APP_LOG_LEVEL", "APP_OBSERVABILITY__LOG_LEVEL")
	if raw == "" {
		return logging.Config{}, ErrLoggingLogLevelIsRequiredSetAPPLOGLEVELOrAPPOBSERVABILITY
	}

	appEnv := strings.ToLower(envFirst("APP_ENV", "APP_APP__ENV"))
	if appEnv == "" {
		appEnv = "local"
	}

	switch appEnv {
	case "local", "staging", "production":
	default:
		return logging.Config{}, fmt.Errorf("logging: invalid environment %q: %w", appEnv, ErrLoggingInvalidEnvironment)
	}

	output := strings.ToLower(envFirst("APP_LOG_OUTPUT", "APP_OBSERVABILITY__LOG_OUTPUT"))
	if output == "" {
		if appEnv == "staging" || appEnv == "production" {
			output = logging.OutputFile
		} else {
			output = logging.OutputStdout
		}
	}

	maxSizeMB, err := envInt(defaultLogMaxSizeMB, "APP_LOG_MAX_SIZE_MB", "APP_OBSERVABILITY__LOG_MAX_SIZE_MB")
	if err != nil {
		return logging.Config{}, err
	}

	maxBackups, err := envInt(defaultLogMaxBackups, "APP_LOG_MAX_BACKUPS", "APP_OBSERVABILITY__LOG_MAX_BACKUPS")
	if err != nil {
		return logging.Config{}, err
	}

	maxAgeDays, err := envInt(defaultLogMaxAgeDays, "APP_LOG_MAX_AGE_DAYS", "APP_OBSERVABILITY__LOG_MAX_AGE_DAYS")
	if err != nil {
		return logging.Config{}, err
	}

	compress, err := envBool(defaultLogCompress, "APP_LOG_COMPRESS", "APP_OBSERVABILITY__LOG_COMPRESS")
	if err != nil {
		return logging.Config{}, err
	}

	cfg := logging.Config{
		Level:      raw,
		Output:     output,
		FilePath:   envFirst("APP_LOG_FILE", "APP_OBSERVABILITY__LOG_FILE"),
		MaxSizeMB:  maxSizeMB,
		MaxBackups: maxBackups,
		MaxAgeDays: maxAgeDays,
		Compress:   compress,
	}
	if err := cfg.Validate(); err != nil {
		return logging.Config{}, err
	}

	return cfg, nil
}

// envFirst returns the first non-blank value among keys. The first key is the
// documented primary name; later names are compatibility/koanf fallbacks.
func envFirst(keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}

	return ""
}

func envInt(fallback int, keys ...string) (int, error) {
	raw := envFirst(keys...)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("logging: invalid %s %q: %w", envLabel(keys), raw, ErrLoggingInvalidEnvironmentValue)
	}

	return value, nil
}

func envBool(fallback bool, keys ...string) (bool, error) {
	raw := envFirst(keys...)
	if raw == "" {
		return fallback, nil
	}

	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("logging: invalid %s %q: %w", envLabel(keys), raw, ErrLoggingInvalidEnvironmentValue)
	}

	return value, nil
}

func envLabel(keys []string) string {
	if len(keys) == 0 {
		return "environment"
	}

	return keys[0]
}

// ProvideLoggerWithConfig is the fx binding for log.Logger: swapping the logging
// backend means changing this one call site (E01-T04-R04). Invalid configs
// fail as errors (fx aborts the graph with the message); callers must not
// pass unvalidated configs — resolve via ProvideLoggingConfig first.
func ProvideLoggerWithConfig(cfg logging.Config) (log.Logger, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return logging.NewFromConfig(cfg)
}

// ProvideClock is the fx binding for kernel.Clock.
func ProvideClock() kernel.Clock {
	return kernel.SystemClock{}
}

// ProvideIDGenerator is the fx binding for kernel.IDGenerator (UUIDv7 per ADR-012).
func ProvideIDGenerator() kernel.IDGenerator {
	return kernel.NewUUIDGenerator()
}
