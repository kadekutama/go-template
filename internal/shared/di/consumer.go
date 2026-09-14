package di

import "go.uber.org/fx"

// ConsumerModule holds NATS-consumer-binary providers (groups land in E14).
func ConsumerModule() fx.Option {
	return fx.Module("consumer")
}
