// Package config owns the single validated Config struct and its layered
// loader (E01-T03). Load priority: base YAML → overlay YAMLs → APP_ env vars
// → secrets (E09-T05; secret references fail closed here).
//
// Environment overrides use the APP_ prefix with __ nesting, lowercased, e.g.
// APP_DATABASE__HOST=pg.internal sets database.host. Dots in key names are
// produced by splitting on __; a single _ stays literal.
package config

import "time"

// Config is the single validated configuration contract for every binary.
// Sections marked `validate:"required"` must be present; scalar fields carry
// their own rules so a bad file fails with every violation listed at once.
type Config struct {
	App           AppConfig           `koanf:"app" validate:"required"`
	Server        ServerConfig        `koanf:"server" validate:"required"`
	Database      DatabaseConfig      `koanf:"database" validate:"required"`
	Cache         CacheConfig         `koanf:"cache" validate:"required"`
	Auth          AuthConfig          `koanf:"auth" validate:"required"`
	HTTPClient    HTTPClientConfig    `koanf:"http_client" validate:"required"`
	NATS          NATSConfig          `koanf:"nats" validate:"required"`
	Redpanda      RedpandaConfig      `koanf:"redpanda" validate:"required"`
	Webhook       WebhookConfig       `koanf:"webhook" validate:"required"`
	Observability ObservabilityConfig `koanf:"observability"`
	FeatureFlags  FeatureFlagConfig   `koanf:"feature_flags"`
	Secrets       SecretsConfig       `koanf:"secrets"`
	Coordination  CoordinationConfig  `koanf:"coordination" validate:"required"`
	Tenancy       TenancyConfig       `koanf:"tenancy" validate:"required"`
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
	Host               string `koanf:"host" validate:"required"`
	Port               int    `koanf:"port" validate:"required,min=1,max=65535"`
	User               string `koanf:"user" validate:"required"`
	Password           string `koanf:"password" validate:"required"`
	Name               string `koanf:"name" validate:"required"`
	SSLMode            string `koanf:"sslmode" validate:"required,oneof=disable require verify-full"`
	MaxOpenConns       int    `koanf:"max_open_conns" validate:"required,min=1"`
	MaxIdleConns       int    `koanf:"max_idle_conns" validate:"required,min=1"`
	ConnMaxLifetimeSec int    `koanf:"conn_max_lifetime_sec" validate:"required,min=1"`
}

// PoolSettings returns effective pool sizing for database/sql.
func (c DatabaseConfig) PoolSettings() (maxOpen, maxIdle int, lifetime time.Duration) {
	return c.MaxOpenConns, c.MaxIdleConns, time.Duration(c.ConnMaxLifetimeSec) * time.Second
}

// CacheConfig points at the Valkey L2 and the hybrid-cache knobs (E08 owns
// L1/L2 behavior).
type CacheConfig struct {
	Host string `koanf:"host" validate:"required"`
	Port int    `koanf:"port" validate:"required,min=1,max=65535"`

	// L1 sizing must be explicitly configured.
	L1MaximumSize int `koanf:"l1_maximum_size" validate:"required,min=1"`

	// Per-data-class staleness bounds in seconds; all required.
	TTLs CacheTTLsConfig `koanf:"ttls" validate:"required"`
}

// CacheTTLsConfig defines per-data-class cache TTLs in seconds.
type CacheTTLsConfig struct {
	BalanceSec         int `koanf:"balance_sec" validate:"required,min=1"`
	BalanceCursorSec   int `koanf:"balance_cursor_sec" validate:"required,min=1"`
	ConfigSec          int `koanf:"config_sec" validate:"required,min=1"`
	ConfigLongSec      int `koanf:"config_long_sec" validate:"required,min=1"`
	FXSec              int `koanf:"fx_sec" validate:"required,min=1"`
	IdempotencyHintSec int `koanf:"idempotency_hint_sec" validate:"required,min=1"`
	RateLimitSec       int `koanf:"rate_limit_sec" validate:"required,min=1"`
	L1PopulateSec      int `koanf:"l1_populate_sec" validate:"required,min=1"`
}

// RedpandaConfig points at the Redpanda event backbone (ADR-014).
type RedpandaConfig struct {
	Brokers       []string `koanf:"brokers" validate:"required,min=1,dive,hostname_port"`
	LedgerEvents  string   `koanf:"topic_ledger_events" validate:"required"`
	OutboxFacts   string   `koanf:"topic_outbox_facts" validate:"required"`
	WebhookJobs   string   `koanf:"topic_webhook_jobs" validate:"required"`
	AuditStreams  string   `koanf:"topic_audit_streams" validate:"required"`
	Partitions    int      `koanf:"partitions" validate:"required,min=1"`
	RetentionHrs  int      `koanf:"retention_hours" validate:"required,min=1"`
	PartitionKeys string   `koanf:"partition_keys" validate:"required"`
	DLQSuffix     string   `koanf:"dlq_suffix" validate:"required"`
}

// WebhookConfig carries the configurable retry backoff (seconds per step) and signature tolerance.
type WebhookConfig struct {
	RetryStepsSec []int `koanf:"retry_steps_sec" validate:"required,min=1,dive,min=1"`
	ToleranceSec  int   `koanf:"tolerance_sec" validate:"required,min=1"`
}

// AuthConfig carries JWT issuer parameters and API key cost settings.
type AuthConfig struct {
	Issuer         string       `koanf:"issuer" validate:"required,url"`
	AccessTTLMin   int          `koanf:"access_ttl_min" validate:"required,min=1"`
	RefreshTTLDays int          `koanf:"refresh_ttl_days" validate:"required,min=1"`
	APIKey         APIKeyConfig `koanf:"api_key" validate:"required"`
}

// APIKeyConfig tunes the Argon2id cost parameters for API keys.
type APIKeyConfig struct {
	Time      int `koanf:"time" validate:"required,min=1,max=16"`
	MemoryKiB int `koanf:"memory_kib" validate:"required,min=8192"`
	Threads   int `koanf:"threads" validate:"required,min=1,max=16"`
	KeyLen    int `koanf:"key_len" validate:"required,min=16,max=64"`
	SaltLen   int `koanf:"salt_len" validate:"required,min=8,max=64"`
}

// HTTPClientConfig tunes the shared outbound HTTP transport.
type HTTPClientConfig struct {
	TimeoutSec          int `koanf:"timeout_sec" validate:"required,min=1"`
	MaxIdleConns        int `koanf:"max_idle_conns" validate:"required,min=1"`
	MaxIdleConnsPerHost int `koanf:"max_idle_conns_per_host" validate:"required,min=1"`
	IdleConnTimeoutSec  int `koanf:"idle_conn_timeout_sec" validate:"required,min=1"`
}

// NATSConfig points at NATS Core (E08/E14 own edge fanout/consumers; ADR-014).
type NATSConfig struct {
	URL string `koanf:"url" validate:"required,url"`
}

// ObservabilityConfig tunes tracing/metrics endpoints (E15 owns the pipeline).
type ObservabilityConfig struct {
	LogLevel       string   `koanf:"log_level" validate:"required,oneof=debug info warn error"`
	OTLPEndpoint   string   `koanf:"otlp_endpoint" validate:"omitempty,url"`
	SampleRatio    float64  `koanf:"sample_ratio" validate:"gt=0,lte=1"`
	ExcludedRoutes []string `koanf:"excluded_routes" validate:"required,min=1,dive,required"`
}

// FeatureFlagConfig points at Unleash (E10 owns the provider).
type FeatureFlagConfig struct {
	UnleashURL string `koanf:"unleash_url" validate:"omitempty,url"`
}

// SecretsConfig locates the secret manager (E09-T05 implements resolution;
// any {{ secret:… }} value fails closed in the E01-T03 loader).
type SecretsConfig struct {
	Provider string `koanf:"provider" validate:"omitempty,oneof=openbao bitwarden env"`
}

// CoordinationConfig points at the etcd coordination plane.
type CoordinationConfig struct {
	EtcdEndpoints          []string `koanf:"etcd_endpoints" validate:"required,min=1"`
	EtcdDialTimeoutSec     int      `koanf:"etcd_dial_timeout_sec" validate:"required,min=1"`
	EtcdElectionTTLSeconds int      `koanf:"etcd_election_ttl_sec" validate:"required,min=1"`
	LeaderKeyPrefix        string   `koanf:"leader_key_prefix" validate:"required"`
}

// TenancyConfig pins tenant-lifecycle policy (E05-T01). E11 wiring maps
// MaxOnboardingAssets into TenantServiceParams; no Go file hardcodes it.
type TenancyConfig struct {
	MaxOnboardingAssets int `koanf:"max_onboarding_assets" validate:"required,min=1"`
}
