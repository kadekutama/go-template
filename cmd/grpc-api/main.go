// Command grpc-api is the gRPC API entrypoint (E01-T02: fx composition; services land in E12).
package main

import (
	"os"

	"github.com/kadekutama/go-template/internal/shared/di"
)

// version identifies the running build. Release pipelines override it with
// -ldflags "-X main.version=$(git describe --tags --always --dirty)" (E17);
// "dev" in output means an unstamped local build is serving traffic.
var version = "dev"

func main() {
	os.Exit(di.RunApp(di.ProvideLogger(), "grpc-api", version, nil,
		di.DomainModule(),
		di.ApplicationModule(),
		di.InfrastructureModule(),
		di.GrpcModule(),
	))
}
