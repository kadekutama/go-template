package featureflag

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the flag adapter in an fx container.
func Module() fx.Option {
	return fx.Module("featureflag",
		fx.Provide(NewClient),
		fx.Provide(func(client *Client) appport.FlagClient {
			return client
		}),
	)
}
