package nats

import (
	"context"

	"go.uber.org/fx"
)

// newPublisherForFX adapts Params-object construction to fx resolution and
// flushes + closes the connection on container stop, so SIGTERM drains
// accepted deltas instead of dropping them. The owning binary provides
// PublisherParams from validated application config.
func newPublisherForFX(lc fx.Lifecycle, params PublisherParams) (*Publisher, error) {
	publisher, err := NewPublisher(params)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return publisher.Close()
		},
	})

	return publisher, nil
}

// Module registers the NATS edge publisher in an fx container. Dialing
// happens in the constructor (fail fast): a process that needs NATS must not
// start without it, and the orchestrator restarts until the broker is
// reachable. Lifecycle owns construction + Close (flush + drain).
func Module() fx.Option {
	return fx.Module("nats",
		fx.Provide(
			newPublisherForFX,
		),
	)
}
