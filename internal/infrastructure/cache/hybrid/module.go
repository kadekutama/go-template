package hybrid

import (
	"go.uber.org/fx"

	appport "github.com/kadekutama/go-template/internal/application/port"
)

// Module registers the shared cache engine in an fx container: Params builds
// *Cache, which is exposed to consumers as both the Engine contract (typed
// views bind to it) and the application port (port consumers). The
// composition root (E14/E17) provides the concrete L1/L2 instances in Params
// (built from validated configuration) and maps port consumers onto the
// port. Typed projections are not registered here: the service that owns a
// projection binds its view with hybrid.Typed[V](engine) at wiring time,
// sharing the engine's instances.
func Module() fx.Option {
	return fx.Module("hybrid-cache",
		fx.Provide(New),
		fx.Provide(func(cache *Cache) Engine {
			return cache
		}),
		fx.Provide(func(cache *Cache) appport.Cache {
			return cache
		}),
	)
}
