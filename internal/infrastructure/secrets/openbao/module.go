package openbao

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the OpenBao store in an fx container.
func Module() fx.Option {
	return fx.Module("openbao",
		fx.Provide(NewStore),
		fx.Provide(func(store *Store) appport.SecretStore {
			return store
		}),
	)
}
