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
	NATS          NATSConfig          `koanf:"nats" validate:"required"`
	Redpanda      RedpandaConfig      `koanf:"redpanda"`
	Webhook       WebhookConfig       `koanf:"webhook"`
	Observability ObservabilityConfig `koanf:"observability"`
	FeatureFlags  FeatureFlagConfig   `koanf:"feature_flags"`
	Secrets       SecretsConfig       `koanf:"secrets"`
	Coordination  CoordinationConfig  `koanf:"coordination"`
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
// Pool knobs are optional: zero values select the postgres package defaults,
// so committed files may omit them until an environment needs tuning.
type DatabaseConfig struct {
	Host               string `koanf:"host" validate:"required"`
	Port               int    `koanf:"port" validate:"required,min=1,max=65535"`
	User               string `koanf:"user" validate:"required"`
	Password           string `koanf:"password" validate:"required"`
	Name               string `koanf:"name" validate:"required"`
	SSLMode            string `koanf:"sslmode" validate:"required,oneof=disable require verify-full"`
	MaxOpenConns       int    `koanf:"max_open_conns" validate:"omitempty,min=1"`
	MaxIdleConns       int    `koanf:"max_idle_conns" validate:"omitempty,min=0"`
	ConnMaxLifetimeSec int    `koanf:"conn_max_lifetime_sec" validate:"omitempty,min=1"`
}

// PoolSettings returns effective pool sizing for database/sql. Zeros mean
// "leave the driver default": postgres.Open already treats non-positive
// values as unset, so this maps 1:1 without inventing new defaults here.
func (c DatabaseConfig) PoolSettings() (maxOpen, maxIdle int, lifetime time.Duration) {
	return c.MaxOpenConns, c.MaxIdleConns, time.Duration(c.ConnMaxLifetimeSec) * time.Second
}

// CacheConfig points at the Valkey L2 and the hybrid-cache knobs (E08 owns
// L1/L2 behavior).
type CacheConfig struct {
	Host string `koanf:"host" validate:"required"`
	Port int    `koanf:"port" validate:"required,min=1,max=65535"`

	// L1 sizing; zero selects the local.OtterCache default.
	L1MaximumSize int `koanf:"l1_maximum_size" validate:"omitempty,min=1"`

	// Per-data-class staleness bounds in seconds; zero values keep the
	// hybrid.DefaultTTLs contract defaults (E14 wiring maps them to Set).
	TTLs CacheTTLsConfig `koanf:"ttls"`
}

// CacheTTLsConfig overrides hybrid.DefaultTTLs per data class (seconds;
// zero values keep the contract defaults).
type CacheTTLsConfig struct {
	BalanceSec         int `koanf:"balance_sec" validate:"omitempty,min=1"`
	BalanceCursorSec   int `koanf:"balance_cursor_sec" validate:"omitempty,min=1"`
	ConfigSec          int `koanf:"config_sec" validate:"omitempty,min=1"`
	ConfigLongSec      int `koanf:"config_long_sec" validate:"omitempty,min=1"`
	FXSec              int `koanf:"fx_sec" validate:"omitempty,min=1"`
	IdempotencyHintSec int `koanf:"idempotency_hint_sec" validate:"omitempty,min=1"`
	RateLimitSec       int `koanf:"rate_limit_sec" validate:"omitempty,min=1"`
	L1PopulateSec      int `koanf:"l1_populate_sec" validate:"omitempty,min=1"`
}

// RedpandaConfig points at the Redpanda event backbone (ADR-014). Topic
// names are optional: zero values keep the ADR-014 contract defaults, and
// E14 wiring maps them to redpanda.TopicsConfig.
type RedpandaConfig struct {
	Brokers      []string `koanf:"brokers" validate:"omitempty,dive,hostname_port"`
	LedgerEvents string   `koanf:"topic_ledger_events" validate:"omitempty"`
	OutboxFacts  string   `koanf:"topic_outbox_facts" validate:"omitempty"`
	WebhookJobs  string   `koanf:"topic_webhook_jobs" validate:"omitempty"`
	AuditStreams string   `koanf:"topic_audit_streams" validate:"omitempty"`
	Partitions   int      `koanf:"partitions" validate:"omitempty,min=1"`
	RetentionHrs int      `koanf:"retention_hours" validate:"omitempty,min=1"`
}

// WebhookConfig carries the configurable retry backoff (seconds per step,
// 1m→5m→15m→1h→6h→24h→48h by default per api-contracts §11).
type WebhookConfig struct {
	RetryStepsSec []int `koanf:"retry_steps_sec" validate:"omitempty,dive,min=1"`
}

// AuthConfig carries JWT issuer parameters (E09 owns key material).
type AuthConfig struct {
	Issuer         string `koanf:"issuer" validate:"required,url"`
	AccessTTLMin   int    `koanf:"access_ttl_min" validate:"required,min=1"`
	RefreshTTLDays int    `koanf:"refresh_ttl_days" validate:"required,min=1"`
}

// NATSConfig points at NATS Core (E08/E14 own edge fanout/consumers; ADR-014).
type NATSConfig struct {
	URL string `koanf:"url" validate:"required,url"`
}

// ObservabilityConfig tunes tracing/metrics endpoints (E15 owns the pipeline).
type ObservabilityConfig struct {
	LogLevel     string  `koanf:"log_level" validate:"omitempty,oneof=debug info warn error"`
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
	Provider string `koanf:"provider" validate:"omitempty,oneof=openbao bitwarden env"`
}

// CoordinationConfig points at the etcd coordination plane (E07.1-T04 owns
// watchers and leader election; zero values mean local-dev defaults).
type CoordinationConfig struct {
	EtcdEndpoints          []string `koanf:"etcd_endpoints"`
	EtcdDialTimeoutSec     int      `koanf:"etcd_dial_timeout_sec" validate:"omitempty,min=1"`
	EtcdElectionTTLSeconds int      `koanf:"etcd_election_ttl_sec" validate:"omitempty,min=1"`
	LeaderKeyPrefix        string   `koanf:"leader_key_prefix"`
}
