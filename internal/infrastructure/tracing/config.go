package tracing

import (
	"errors"
	"fmt"
	"strings"
)

// Config tunes the OTel SDK bootstrap.
// All required fields must be explicitly populated.
type Config struct {
	// ServiceName and ServiceVersion identify the resource (required).
	ServiceName    string
	ServiceVersion string
	// Env is local/staging/production. Production REQUIRES OTLPEndpoint.
	Env string
	// OTLPEndpoint is an https?://host:port OTLP/gRPC endpoint. Empty disables
	// export outside production (local development default).
	OTLPEndpoint string
	// SampleRatio in (0,1] is required.
	SampleRatio float64
	// ExcludedRoutes defines routes that must never be traced (e.g., health probes). Required.
	ExcludedRoutes []string
}

// Validate ensures all required configuration values are present and well-formed.
func (c Config) Validate() error {
	if strings.TrimSpace(c.ServiceName) == "" {
		return errors.New("tracing: ServiceName is required")
	}
	if strings.TrimSpace(c.ServiceVersion) == "" {
		return errors.New("tracing: ServiceVersion is required")
	}
	if c.SampleRatio <= 0 || c.SampleRatio > 1 {
		return fmt.Errorf("tracing: SampleRatio %v out of (0,1]", c.SampleRatio)
	}
	if len(c.ExcludedRoutes) == 0 {
		return errors.New("tracing: ExcludedRoutes is required")
	}
	for _, r := range c.ExcludedRoutes {
		if strings.TrimSpace(r) == "" {
			return errors.New("tracing: excluded route must not be blank")
		}
	}
	if strings.TrimSpace(c.OTLPEndpoint) == "" && strings.EqualFold(c.Env, "production") {
		return errors.New("tracing: production requires OTLPEndpoint (refusing silent no-op)")
	}
	return nil
}
