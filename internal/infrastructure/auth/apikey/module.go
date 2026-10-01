package apikey

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the API key manager in an fx container.
func Module() fx.Option {
	return fx.Module("apikey",
		fx.Provide(NewManager),
		fx.Provide(func(manager *Manager) appport.APIKeyStore {
			return manager
		}),
	)
}
