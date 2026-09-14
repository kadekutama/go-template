// Package config owns the single validated Config struct and its layered
// loader (E01-T03). Load priority: base YAML → overlay YAMLs → APP_ env vars
// → secrets (E09-T05; secret references fail closed here).
//
// Environment overrides use the APP_ prefix with __ nesting, lowercased, e.g.
// APP_DATABASE__HOST=pg.internal sets database.host. Dots in key names are
// produced by splitting on __; a single _ stays literal.
package config

// Config is the single validated configuration contract for every binary.
// Sections marked `validate:"required"` must be present; scalar fields carry
// their own rules so a bad file fails with every violation listed at once.
type Config struct {
	App           AppConfig           `koanf:"app" validate:"required"`
	Server        ServerConfig        `koanf:"server" validate:"required"`
	Database      DatabaseConfig      `koanf:"database" validate:"required"`
	Cache         CacheConfig         `koanf:"cache" validate:"required"`
	Auth          AuthConfig          `koanf:"auth" validate:"required"`
	NATS          NATSConfig          `koanf:"nats" validate:"required"`
	Observability ObservabilityConfig `koanf:"observability"`
	FeatureFlags  FeatureFlagConfig   `koanf:"feature_flags"`
	Secrets       SecretsConfig       `koanf:"secrets"`
}

// AppConfig identifies the deployment.
type AppConfig struct {
	Name string `koanf:"name" validate:"required"`
	Env  string `koanf:"env" validate:"required,oneof=local staging production"`
}

// ServerConfig tunes the API servers.
type ServerConfig struct {
	Host string `koanf:"host" validate:"required,hostname|ip"`
	Port int    `koanf:"port" validate:"required,min=1,max=65535"`
}

// DatabaseConfig points at PostgreSQL (E07 owns pooling/migration behavior).
type DatabaseConfig struct {
	Host     string `koanf:"host" validate:"required"`
	Port     int    `koanf:"port" validate:"required,min=1,max=65535"`
	User     string `koanf:"user" validate:"required"`
	Password string `koanf:"password" validate:"required"`
	Name     string `koanf:"name" validate:"required"`
	SSLMode  string `koanf:"sslmode" validate:"required,oneof=disable require verify-full"`
}

// CacheConfig points at the Valkey L2 (E08 owns L1/L2 behavior).
type CacheConfig struct {
	Host string `koanf:"host" validate:"required"`
	Port int    `koanf:"port" validate:"required,min=1,max=65535"`
}

// AuthConfig carries JWT issuer parameters (E09 owns key material).
type AuthConfig struct {
	Issuer         string `koanf:"issuer" validate:"required,url"`
	AccessTTLMin   int    `koanf:"access_ttl_min" validate:"required,min=1"`
	RefreshTTLDays int    `koanf:"refresh_ttl_days" validate:"required,min=1"`
}

// NATSConfig points at NATS JetStream (E08/E14 own topology/consumers).
type NATSConfig struct {
	URL string `koanf:"url" validate:"required,url"`
}

// ObservabilityConfig tunes tracing/metrics endpoints (E15 owns the pipeline).
type ObservabilityConfig struct {
	OTLPEndpoint string  `koanf:"otlp_endpoint" validate:"omitempty,url"`
	SampleRatio  float64 `koanf:"sample_ratio" validate:"min=0,max=1"`
}

// FeatureFlagConfig points at Unleash (E10 owns the provider).
type FeatureFlagConfig struct {
	UnleashURL string `koanf:"unleash_url" validate:"omitempty,url"`
}

// SecretsConfig locates the secret manager (E09-T05 implements resolution;
// any {{ secret:… }} value fails closed in the E01-T03 loader).
type SecretsConfig struct {
	Provider string `koanf:"provider" validate:"omitempty,oneof=bitwarden env"`
}
