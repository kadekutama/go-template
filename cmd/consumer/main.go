// Command consumer is the NATS consumer entrypoint (E01-T02: fx composition; groups land in E14).
package main

import (
	"example.com/go-template/internal/shared/di"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		di.DomainModule(),
		di.ApplicationModule(),
		di.InfrastructureModule(),
		di.ConsumerModule(),
		fx.NopLogger,
	).Run()
}
