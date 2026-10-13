// Package etcd is the etcd v3.7.0 coordination adapter (E07.1-T04): gRPC
// streaming config watchers and single-writer worker leader election behind
// small consumer-owned interfaces. It implements ADR-017 over
// go.etcd.io/etcd/client/v3 and registers an fx module for composition.
package etcd

import (
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
		return ErrEtcdAtLeastOneEndpointIsRequired
	}

	for _, endpoint := range c.Endpoints {
		if strings.TrimSpace(endpoint) == "" {
			return ErrEtcdEndpointMustNotBeBlank
		}
	}

	if c.DialTimeout <= 0 {
		return ErrEtcdDialTimeoutMustBePositive
	}

	if c.ElectionTTLSeconds <= 0 {
		return ErrEtcdElectionTTLMustBePositive
	}

	if strings.TrimSpace(c.LeaderKeyPrefix) == "" {
		return ErrEtcdLeaderKeyPrefixMustNotBeBlank
	}

	return nil
}
