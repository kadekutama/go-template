package config

import (
	"fmt"
	"time"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
)

// This file maps validated configuration sections onto adapter constructors'
// parameter objects. Composition roots (E14 workers, cmd binaries) call
// these instead of reading YAML keys or adapter constants directly, so every
// tunable value has exactly one config-owned source.

// HybridTTLs maps the per-data-class seconds onto hybrid.TTLSet: zero fields
// keep the contract defaults, then the result is validated so a negative or
// nonsensical value fails at wiring time instead of caching without expiry.
func (c CacheTTLsConfig) HybridTTLs() (hybrid.TTLSet, error) {
	ttl, err := hybrid.TTLSet{
		Balance:         time.Duration(c.BalanceSec) * time.Second,
		BalanceCursor:   time.Duration(c.BalanceCursorSec) * time.Second,
		Config:          time.Duration(c.ConfigSec) * time.Second,
		ConfigLong:      time.Duration(c.ConfigLongSec) * time.Second,
		FX:              time.Duration(c.FXSec) * time.Second,
		IdempotencyHint: time.Duration(c.IdempotencyHintSec) * time.Second,
		RateLimit:       time.Duration(c.RateLimitSec) * time.Second,
		L1Populate:      time.Duration(c.L1PopulateSec) * time.Second,
	}.WithDefaults()
	if err != nil {
		return hybrid.TTLSet{}, err
	}

	if err := ttl.Validate(); err != nil {
		return hybrid.TTLSet{}, err
	}

	return ttl, nil
}

// Topics maps the optional topic-name overrides onto redpanda.TopicsConfig;
// zero values keep the ADR-014 contract defaults (applied here so callers
// receive a fully resolved configuration).
func (c RedpandaConfig) Topics() redpanda.TopicsConfig {
	return redpanda.TopicsConfig{
		LedgerEvents: c.LedgerEvents,
		OutboxFacts:  c.OutboxFacts,
		WebhookJobs:  c.WebhookJobs,
		AuditStreams: c.AuditStreams,
		Partitions:   c.Partitions,
		RetentionHrs: c.RetentionHrs,
	}.WithDefaults()
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
