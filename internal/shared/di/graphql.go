package di

import "go.uber.org/fx"

// GraphQLModule holds GraphQL-binary providers (resolvers land in E13).
func GraphQLModule() fx.Option {
	return fx.Module("graphql")
}
