package crypto

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the envelope crypto adapter in an fx container.
func Module() fx.Option {
	return fx.Module("crypto",
		fx.Provide(NewEnveloper),
		fx.Provide(func(enveloper *Enveloper) appport.EnvelopeCrypto {
			return enveloper
		}),
	)
}
