// Package etcd is the etcd v3.7.0 coordination adapter (E07.1-T04): gRPC
// streaming config watchers and single-writer worker leader election behind
// small consumer-owned interfaces. It implements ADR-017 over
// go.etcd.io/etcd/client/v3 and registers an fx module for composition.
package etcd

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultDialTimeout bounds initial etcd connection establishment.
const DefaultDialTimeout = 5 * time.Second

// DefaultElectionTTLSeconds is the default leader-election lease TTL.
const DefaultElectionTTLSeconds = 5

// DefaultLeaderKeyPrefix is the default election key prefix for
// single-writer financial workers (reconciliation, period close).
const DefaultLeaderKeyPrefix = "/finance/worker-leader"

// Config tunes the etcd adapter. Endpoints are required; every other field
// has a safe default applied by withDefaults.
type Config struct {
	Endpoints          []string
	DialTimeout        time.Duration
	Username           string
	Password           string
	ElectionTTLSeconds int
	LeaderKeyPrefix    string
}

// withDefaults returns a copy of c with zero values replaced by defaults.
func (c Config) withDefaults() Config {
	if c.DialTimeout <= 0 {
		c.DialTimeout = DefaultDialTimeout
	}

	if c.ElectionTTLSeconds <= 0 {
		c.ElectionTTLSeconds = DefaultElectionTTLSeconds
	}

	if strings.TrimSpace(c.LeaderKeyPrefix) == "" {
		c.LeaderKeyPrefix = DefaultLeaderKeyPrefix
	}

	return c
}

// DefaultConfig returns connection defaults with APP_COORDINATION__*
// process-environment overlays applied on top, so endpoints, timeouts, and
// the leader prefix are customizable without code changes:
//
//	APP_COORDINATION__ETCD_ENDPOINTS        comma-separated URLs
//	APP_COORDINATION__ETCD_DIAL_TIMEOUT_SEC seconds, positive
//	APP_COORDINATION__ETCD_ELECTION_TTL_SEC seconds, positive
//	APP_COORDINATION__ETCD_LEADER_PREFIX    non-blank key prefix
//
// Blank or malformed values keep the compiled default for that field and are
// never fatal: a half-set environment still boots against localhost rather
// than refusing to start. Production overlays may also map validated
// application config through ConfigForEndpoints at the composition root.
func DefaultConfig() Config {
	cfg := Config{
		Endpoints:          []string{"http://127.0.0.1:2379"},
		DialTimeout:        DefaultDialTimeout,
		ElectionTTLSeconds: DefaultElectionTTLSeconds,
		LeaderKeyPrefix:    DefaultLeaderKeyPrefix,
	}

	if raw := strings.TrimSpace(os.Getenv("APP_COORDINATION__ETCD_ENDPOINTS")); raw != "" {
		endpoints := make([]string, 0, 3)
		for _, endpoint := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(endpoint); trimmed != "" {
				endpoints = append(endpoints, trimmed)
			}
		}

		if len(endpoints) > 0 {
			cfg.Endpoints = endpoints
		}
	}

	if seconds, ok := envPositiveInt("APP_COORDINATION__ETCD_DIAL_TIMEOUT_SEC"); ok {
		cfg.DialTimeout = time.Duration(seconds) * time.Second
	}

	if ttl, ok := envPositiveInt("APP_COORDINATION__ETCD_ELECTION_TTL_SEC"); ok {
		cfg.ElectionTTLSeconds = ttl
	}

	if prefix := strings.TrimSpace(os.Getenv("APP_COORDINATION__ETCD_LEADER_PREFIX")); prefix != "" {
		cfg.LeaderKeyPrefix = prefix
	}

	return cfg
}

// envPositiveInt reads a positive integer environment value.
func envPositiveInt(key string) (int, bool) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return 0, false
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, false
	}

	return value, true
}

// Validate reports whether c can initialize an etcd client.
func (c Config) Validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("etcd: at least one endpoint is required")
	}

	for _, endpoint := range c.Endpoints {
		if strings.TrimSpace(endpoint) == "" {
			return fmt.Errorf("etcd: endpoint must not be blank")
		}
	}

	if c.DialTimeout <= 0 {
		return fmt.Errorf("etcd: dial timeout must be positive")
	}

	if c.ElectionTTLSeconds <= 0 {
		return fmt.Errorf("etcd: election TTL must be positive")
	}

	if strings.TrimSpace(c.LeaderKeyPrefix) == "" {
		return fmt.Errorf("etcd: leader key prefix must not be blank")
	}

	return nil
}
