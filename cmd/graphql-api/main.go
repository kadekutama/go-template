// Command graphql-api is the GraphQL API entrypoint (E01-T02: fx composition; resolvers land in E13).
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
		di.GraphQLModule(),
		fx.NopLogger,
	).Run()
}
