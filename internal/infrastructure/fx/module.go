package fx

import (
	uberfx "go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the FX adapter in an fx container. E11/E14 composition
// roots supply Params; this task validates the graph with fx.ValidateApp but
// does not register the module in the shared root.
func Module() uberfx.Option {
	return uberfx.Module("fx",
		uberfx.Provide(NewProvider),
		uberfx.Provide(func(provider *Provider) appport.FxRateProvider {
			return provider
		}),
	)
}
