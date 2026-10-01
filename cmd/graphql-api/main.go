// Command graphql-api is the GraphQL API entrypoint (E01-T02: fx composition; resolvers land in E13).
package main

import (
	"fmt"
	"os"

	"github.com/kadekutama/go-template/internal/shared/di"
)

// version identifies the running build. Release pipelines override it with
// -ldflags "-X main.version=$(git describe --tags --always --dirty)" (E17);
// "dev" in output means an unstamped local build is serving traffic.
var version = "dev"

func main() {
	cfg, err := di.ProvideLoggingConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "graphql-api: "+err.Error())
		os.Exit(1)
	}
	logger, err := di.ProvideLoggerWithConfig(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "graphql-api: "+err.Error())
		os.Exit(1)
	}
	os.Exit(di.RunApp(logger, "graphql-api", version, nil,
		di.DomainModule(),
		di.ApplicationModule(),
		di.InfrastructureModule(),
		di.GraphQLModule(),
	))
}
