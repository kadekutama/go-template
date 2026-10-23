// Package profiling is the continuous-profiling infrastructure adapter
// (ADR-027). It owns the Pyroscope SDK and the OpenTelemetry span-to-profile
// bridge; no SDK type leaks into the domain or application layers.
//
// The adapter is disabled by default. Disabled Start/Stop are zero-allocation
// no-ops so unit tests and local development never open network connections or
// mutate process-global runtime profiling rates.
package profiling

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

// Config tunes the continuous profiler. The koanf/json tags keep the app
// config section and this adapter's constructor contract aligned; validation
// is shared through the kernel validate helper.
type Config struct {
	Enabled              bool              `json:"enabled"                koanf:"enabled"`
	ServerAddress        string            `json:"server_address"         koanf:"server_address"         validate:"required_if=Enabled true,omitempty,url"`
	ApplicationName      string            `json:"application_name"       koanf:"application_name"       validate:"required_if=Enabled true"`
	Environment          string            `json:"environment"            koanf:"environment"`
	UploadRate           time.Duration     `json:"upload_rate"            koanf:"upload_rate"            validate:"min=0"`
	MutexProfileFraction int               `json:"mutex_profile_fraction" koanf:"mutex_profile_fraction" validate:"min=0"`
	BlockProfileRate     int               `json:"block_profile_rate"     koanf:"block_profile_rate"     validate:"min=0"`
	DisableGCRuns        bool              `json:"disable_gc_runs"        koanf:"disable_gc_runs"`
	Tags                 map[string]string `json:"tags"                   koanf:"tags"`
}

// Validate checks data fields and the static tag policy. Dependency seams are
// checked explicitly by NewProfiler (nil logger), following the repository's
// Parameter Object convention.
func (c Config) Validate() error {
	if err := validate.Struct("profiling", "config", c); err != nil {
		return err
	}

	for key := range c.Tags {
		if strings.TrimSpace(key) == "" {
			return ErrEmptyTagKey
		}

		if isForbiddenTagKey(key) {
			return fmt.Errorf("%w: %q", ErrForbiddenTagKey, key)
		}

		if !isAllowedTagKey(key) {
			return fmt.Errorf("%w: %q", ErrUnsupportedTagKey, key)
		}
	}

	return nil
}

// clone detaches the caller's tag map so later mutation cannot change a
// running profiler's labels.
func (c Config) clone() Config {
	out := c
	out.Tags = maps.Clone(c.Tags)

	return out
}

// isForbiddenTagKey enforces the ADR-027 high-cardinality/zero-PII policy.
// Matching is case-insensitive and ignores surrounding whitespace.
func isForbiddenTagKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "tenant_id", "account_id", "user_id", "email", "pan":
		return true
	default:
		return false
	}
}

// isAllowedTagKey limits static tags to the ADR-027 low-cardinality set.
func isAllowedTagKey(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "service", "env", "region", "version":
		return true
	default:
		return false
	}
}
