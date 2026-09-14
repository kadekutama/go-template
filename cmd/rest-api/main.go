// Command rest-api is the REST API entrypoint (E01-T02: fx composition; handlers land in E11).
package main

import (
	"github.com/kadekutama/go-template/internal/shared/di"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		di.DomainModule(),
		di.ApplicationModule(),
		di.InfrastructureModule(),
		di.RestModule(),
		fx.NopLogger,
	).Run()
}
