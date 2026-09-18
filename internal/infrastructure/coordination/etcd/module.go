package etcd

import (
	"context"
	"time"

	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// ConfigForEndpoints builds a Config from deployment values, applying safe
// defaults for non-positive timeouts/TTLs and blank prefixes. The owning
// binary maps its validated application config through this helper instead
// of hand-rolling endpoint plumbing.
func ConfigForEndpoints(endpoints []string, dialTimeout time.Duration, electionTTLSeconds int) Config {
	cfg := Config{
		Endpoints:          endpoints,
		DialTimeout:        dialTimeout,
		ElectionTTLSeconds: electionTTLSeconds,
		LeaderKeyPrefix:    DefaultLeaderKeyPrefix,
	}

	return cfg.withDefaults()
}

// newClientForFX adapts Params-object construction to fx resolution and
// drains the client on container stop so SIGTERM closes etcd connections
// instead of leaking them.
func newClientForFX(lc fx.Lifecycle, cfg Config) (*Client, error) {
	client, err := NewClient(ClientParams{Config: cfg})
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}

// newWatcherForFX adapts Params-object construction to fx resolution.
func newWatcherForFX(client *Client, logger log.Logger) (Watcher, error) {
	return NewWatcher(WatcherParams{Client: client, Logger: logger})
}

// newElectionForFX adapts Config-bound construction to fx resolution.
func newElectionForFX(client *Client, cfg Config) (LeaderElector, error) {
	resolved := cfg.withDefaults()

	return NewElection(ElectionParams{
		Client:     client,
		KeyPrefix:  resolved.LeaderKeyPrefix,
		TTLSeconds: resolved.ElectionTTLSeconds,
	})
}

// Module registers the etcd coordination adapter in an fx container.
//
// The module is opt-in per binary (it is intentionally not part of the
// shared infrastructure module): only the worker/consumer binaries that
// campaign or watch include it, mapping their validated application config
// through ConfigForEndpoints. E14 owns that wiring.
func Module() fx.Option {
	return fx.Module("etcd",
		fx.Provide(
			DefaultConfig,
			newClientForFX,
			newWatcherForFX,
			newElectionForFX,
		),
	)
}
