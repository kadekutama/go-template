package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/infrastructure/auth/apikey"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/coordination/etcd"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/tracing"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
)

// This file maps validated configuration sections onto adapter constructors'
// parameter objects. Composition roots (E14 workers, cmd binaries) call
// these instead of reading YAML keys or adapter constants directly, so every
// tunable value has exactly one config-owned source.

// HybridTTLs maps the per-data-class seconds onto hybrid.TTLSet: every field
// must be explicitly configured and positive, failing at wiring time otherwise.
func (c CacheTTLsConfig) HybridTTLs() (hybrid.TTLSet, error) {
	ttl := hybrid.TTLSet{
		Balance:         time.Duration(c.BalanceSec) * time.Second,
		BalanceCursor:   time.Duration(c.BalanceCursorSec) * time.Second,
		Config:          time.Duration(c.ConfigSec) * time.Second,
		ConfigLong:      time.Duration(c.ConfigLongSec) * time.Second,
		FX:              time.Duration(c.FXSec) * time.Second,
		IdempotencyHint: time.Duration(c.IdempotencyHintSec) * time.Second,
		RateLimit:       time.Duration(c.RateLimitSec) * time.Second,
		L1Populate:      time.Duration(c.L1PopulateSec) * time.Second,
	}

	if err := ttl.Validate(); err != nil {
		return hybrid.TTLSet{}, err
	}

	return ttl, nil
}

// AssetCap returns the configured per-call asset cap for tenant
// onboarding (E05-T01-R03). Explicit and positive; wiring fails otherwise so
// the product invariant never rests on test convention.
func (c TenancyConfig) AssetCap() (int, error) {
	if c.MaxOnboardingAssets <= 0 {
		return 0, fmt.Errorf("tenancy: max_onboarding_assets must be positive (see config.yaml)")
	}
	return c.MaxOnboardingAssets, nil
}

// Topics maps the topic configuration onto redpanda.TopicsConfig;
// all topic names and sizing must be explicitly configured and valid.
func (c RedpandaConfig) Topics() (redpanda.TopicsConfig, error) {
	cfg := redpanda.TopicsConfig{
		LedgerEvents:  c.LedgerEvents,
		OutboxFacts:   c.OutboxFacts,
		WebhookJobs:   c.WebhookJobs,
		AuditStreams:  c.AuditStreams,
		Partitions:    c.Partitions,
		RetentionHrs:  c.RetentionHrs,
		PartitionKeys: c.PartitionKeys,
		DLQSuffix:     c.DLQSuffix,
	}

	if err := cfg.Validate(); err != nil {
		return redpanda.TopicsConfig{}, err
	}

	return cfg, nil
}

// RetryPolicy maps the configured per-step seconds onto a webhook retry
// policy. There is no code default: an empty list fails at wiring time so a
// missing schedule can never silently fall back to a hardcoded table.
func (c WebhookConfig) RetryPolicy() (*webhook.RetryPolicy, error) {
	if len(c.RetryStepsSec) == 0 {
		return nil, fmt.Errorf("webhook: retry steps are required (see config.yaml)")
	}

	steps := make([]time.Duration, 0, len(c.RetryStepsSec))
	for _, seconds := range c.RetryStepsSec {
		steps = append(steps, time.Duration(seconds)*time.Second)
	}

	return webhook.NewRetryPolicy(steps)
}

// Tolerance returns the configured signature timestamp tolerance.
func (c WebhookConfig) Tolerance() (time.Duration, error) {
	if c.ToleranceSec <= 0 {
		return 0, fmt.Errorf("webhook: tolerance_sec must be positive (see config.yaml)")
	}
	return time.Duration(c.ToleranceSec) * time.Second, nil
}

// Coordination maps the configured coordination parameters onto etcd.Config.
// All fields must be explicitly populated and valid.
func (c CoordinationConfig) Coordination() (etcd.Config, error) {
	cfg := etcd.Config{
		Endpoints:          append([]string(nil), c.EtcdEndpoints...),
		DialTimeout:        time.Duration(c.EtcdDialTimeoutSec) * time.Second,
		ElectionTTLSeconds: c.EtcdElectionTTLSeconds,
		LeaderKeyPrefix:    c.LeaderKeyPrefix,
	}

	if err := cfg.Validate(); err != nil {
		return etcd.Config{}, err
	}

	return cfg, nil
}

// HTTPClientParams maps the configured HTTP client parameters onto httpclient.Params.
func (c HTTPClientConfig) HTTPClientParams() (httpclient.Params, error) {
	if c.TimeoutSec <= 0 || c.MaxIdleConns <= 0 || c.MaxIdleConnsPerHost <= 0 || c.IdleConnTimeoutSec <= 0 {
		return httpclient.Params{}, fmt.Errorf("httpclient: all parameters must be positive")
	}

	return httpclient.Params{
		Timeout:             time.Duration(c.TimeoutSec) * time.Second,
		MaxIdleConns:        c.MaxIdleConns,
		MaxIdleConnsPerHost: c.MaxIdleConnsPerHost,
		IdleConnTimeout:     time.Duration(c.IdleConnTimeoutSec) * time.Second,
	}, nil
}

// HasherParams maps the configured Argon2 parameters onto apikey.HasherParams.
// The thread ceiling (16) matches APIKeyConfig's `max=16` tag so file
// validation and mapping agree on every bound.
func (c APIKeyConfig) HasherParams() (apikey.HasherParams, error) {
	if c.Time <= 0 || c.MemoryKiB <= 0 || c.Threads <= 0 || c.Threads > 16 || c.KeyLen <= 0 || c.SaltLen <= 0 {
		return apikey.HasherParams{}, fmt.Errorf("apikey: all parameters must be positive (threads 1-16)")
	}

	return apikey.HasherParams{
		Time:    uint32(c.Time),      //nolint:gosec // guarded positive above
		Memory:  uint32(c.MemoryKiB), //nolint:gosec // guarded positive above
		Threads: uint8(c.Threads),    //nolint:gosec // guarded <= 255 above
		KeyLen:  uint32(c.KeyLen),    //nolint:gosec // guarded positive above
		SaltLen: c.SaltLen,
	}, nil
}

// Tracing maps app identity + observability onto tracing.Config for E11 wiring.
// serviceVersion is release-stamped at the binary edge (ldflags); everything
// else comes from config. Validation failures name the offending field.
func (c Config) Tracing(serviceVersion string) (tracing.Config, error) {
	cfg := tracing.Config{
		ServiceName:    c.App.Name,
		ServiceVersion: serviceVersion,
		Env:            c.App.Env,
		OTLPEndpoint:   c.Observability.OTLPEndpoint,
		SampleRatio:    c.Observability.SampleRatio,
		ExcludedRoutes: append([]string(nil), c.Observability.ExcludedRoutes...),
	}
	if err := cfg.Validate(); err != nil {
		return tracing.Config{}, err
	}
	return cfg, nil
}

// Logging maps the configured log level onto logging.Config.
func (c ObservabilityConfig) Logging() (logging.Config, error) {
	if strings.TrimSpace(c.LogLevel) == "" {
		return logging.Config{}, fmt.Errorf("logging: log_level is required")
	}

	cfg := logging.Config{Level: c.LogLevel}
	if err := cfg.Validate(); err != nil {
		return logging.Config{}, err
	}

	return cfg, nil
}
