package tracing

// DefaultSampleRatio is the head-sampling probability when Config leaves it unset.
const DefaultSampleRatio = 0.10

// DefaultExcludedRoutes are never traced by E11/E12/E13 middleware.
var DefaultExcludedRoutes = []string{"/healthz", "/readyz", "/livez"}

// Config tunes the OTel SDK bootstrap.
type Config struct {
	// ServiceName and ServiceVersion identify the resource (required).
	ServiceName    string
	ServiceVersion string
	// Env is local/staging/production. Production REQUIRES OTLPEndpoint.
	Env string
	// OTLPEndpoint is an https?://host:port OTLP/gRPC endpoint. Empty disables
	// export outside production (local development default).
	OTLPEndpoint string
	// SampleRatio in [0,1]; values <= 0 select DefaultSampleRatio.
	SampleRatio float64
}
