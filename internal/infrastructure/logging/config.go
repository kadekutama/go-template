package logging

import (
	"fmt"
	"strings"
)

// Output modes supported by the logging adapter.
const (
	OutputStdout = "stdout"
	OutputFile   = "file"
	OutputBoth   = "both"
)

// Config tunes the adapter.
type Config struct {
	Level      string
	Output     string
	FilePath   string
	MaxSizeMB  int
	MaxBackups int
	MaxAgeDays int
	Compress   bool
}

// Normalized lowercases the controlled values and applies the backward-
// compatible empty output mode. Validation decides rotation bounds: file mode
// requires a positive maximum size, while zero backups or age means unlimited
// retention in lumberjack.
func (c Config) Normalized() Config {
	c.Level = strings.ToLower(strings.TrimSpace(c.Level))
	c.Output = strings.ToLower(strings.TrimSpace(c.Output))
	c.FilePath = strings.TrimSpace(c.FilePath)

	if c.Output == "" {
		c.Output = OutputStdout
	}

	return c
}

// Validate ensures the log level and output mode are explicit and valid.
// "disabled" silences all output (zerolog.Disabled) and is legal for tests and
// quiet workers; every other level must name a real level. File output
// requires a path and a positive maximum size; backup and age bounds may be
// zero, which lumberjack interprets as unlimited retention.
func (c Config) Validate() error {
	c = c.Normalized()

	switch c.Level {
	case "":
		return ErrLoggingLevelIsRequired
	case "debug", "info", "warn", "error", "disabled":
	default:
		return fmt.Errorf("logging: unknown log level %q: %w", c.Level, ErrLoggingUnknownLogLevel)
	}

	switch c.Output {
	case OutputStdout, OutputFile, OutputBoth:
	default:
		return fmt.Errorf("logging: unknown output mode %q: %w", c.Output, ErrLoggingUnknownOutputMode)
	}

	if c.MaxSizeMB < 0 || c.MaxBackups < 0 || c.MaxAgeDays < 0 {
		return ErrLoggingInvalidRotation
	}

	if c.Output == OutputFile || c.Output == OutputBoth {
		if c.FilePath == "" {
			return ErrLoggingFilePathIsRequired
		}

		if c.MaxSizeMB == 0 {
			return ErrLoggingInvalidRotation
		}
	}

	return nil
}
