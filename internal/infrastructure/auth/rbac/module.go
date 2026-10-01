package rbac

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the RBAC enforcer in an fx container.
func Module() fx.Option {
	return fx.Module("rbac",
		fx.Provide(NewEnforcer),
		fx.Provide(func(enforcer *Enforcer) appport.Authorizer {
			return enforcer
		}),
	)
}
