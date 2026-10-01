// Package etcd is the etcd v3.7.0 coordination adapter (E07.1-T04): gRPC
// streaming config watchers and single-writer worker leader election behind
// small consumer-owned interfaces. It implements ADR-017 over
// go.etcd.io/etcd/client/v3 and registers an fx module for composition.
package etcd

import (
	"fmt"
	"strings"
	"time"
)

// Config tunes the etcd adapter. All fields must be explicitly configured.
type Config struct {
	Endpoints          []string
	DialTimeout        time.Duration
	Username           string
	Password           string
	ElectionTTLSeconds int
	LeaderKeyPrefix    string
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
