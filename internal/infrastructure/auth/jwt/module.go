package jwt

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the JWT issuer in an fx container.
func Module() fx.Option {
	return fx.Module("jwt",
		fx.Provide(NewIssuer),
		fx.Provide(func(issuer *Issuer) appport.TokenIssuer {
			return issuer
		}),
	)
}
