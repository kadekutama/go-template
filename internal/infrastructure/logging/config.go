package logging

import (
	"errors"
	"fmt"
	"strings"
)

// Config tunes the adapter.
type Config struct {
	Level string
}

// Validate ensures the log level is explicit and valid. "disabled" silences
// all output (zerolog.Disabled) and is legal for tests and quiet workers;
// every other value must name a real level.
func (c Config) Validate() error {
	trimmed := strings.ToLower(strings.TrimSpace(c.Level))
	if trimmed == "" {
		return errors.New("logging: level is required")
	}
	switch trimmed {
	case "debug", "info", "warn", "error", "disabled":
		return nil
	default:
		return fmt.Errorf("logging: unknown log level %q", c.Level)
	}
}
