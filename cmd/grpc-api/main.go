// Command grpc-api is the gRPC API entrypoint (E01-T02: fx composition; services land in E12).
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
		di.GrpcModule(),
		fx.NopLogger,
	).Run()
}
