package oauth2

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/valkey"
)

// Module registers the OAuth provider in an fx container.
func Module() fx.Option {
	return fx.Module("oauth2",
		fx.Provide(func(client *valkey.ValkeyClient) KVStore {
			return client
		}),
		fx.Provide(NewProvider),
		fx.Provide(func(provider *Provider) appport.OAuthProvider {
			return provider
		}),
	)
}
