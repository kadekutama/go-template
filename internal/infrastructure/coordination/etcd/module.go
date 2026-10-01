package etcd

import (
	"context"
	"time"

	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// ConfigForEndpoints builds a Config from deployment values and validates it.
// The owning binary maps its validated application config through this helper
// instead of hand-rolling endpoint plumbing.
func ConfigForEndpoints(endpoints []string, dialTimeout time.Duration, electionTTLSeconds int, leaderKeyPrefix string) (Config, error) {
	cfg := Config{
		Endpoints:          endpoints,
		DialTimeout:        dialTimeout,
		ElectionTTLSeconds: electionTTLSeconds,
		LeaderKeyPrefix:    leaderKeyPrefix,
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
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
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return NewElection(ElectionParams{
		Client:     client,
		KeyPrefix:  cfg.LeaderKeyPrefix,
		TTLSeconds: cfg.ElectionTTLSeconds,
	})
}

// Module registers the etcd coordination adapter in an fx container.
//
// The module is opt-in per binary (it is intentionally not part of the
// shared infrastructure module): only the worker/consumer binaries that
// campaign or watch include it, mapping their validated application config
// through CoordinationConfig.Coordination(). E14 owns that wiring.
func Module() fx.Option {
	return fx.Module("etcd",
		fx.Provide(
			newClientForFX,
			newWatcherForFX,
			newElectionForFX,
		),
	)
}
