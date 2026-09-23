package redpanda

import (
	"context"

	"go.uber.org/fx"
)

// newProducerForFX adapts Params-object construction to fx resolution and
// flushes + closes the producer on container stop, so SIGTERM drains
// in-flight records instead of losing acknowledged publishes. The owning
// binary provides ProducerParams from validated application config.
func newProducerForFX(lc fx.Lifecycle, params ProducerParams) (*Producer, error) {
	producer, err := NewProducer(params)
	if err != nil {
		return nil, err
	}

	lc.Append(fx.Hook{
		OnStop: func(context.Context) error {
			return producer.Close()
		},
	})

	return producer, nil
}

// Module registers the Redpanda producer in an fx container. The producer
// dials lazily on first publish (fail at first publish, not at
// construction), so container startup never blocks on broker reachability;
// lifecycle owns construction + Close (flush + drain).
func Module() fx.Option {
	return fx.Module("redpanda",
		fx.Provide(
			newProducerForFX,
		),
	)
}
