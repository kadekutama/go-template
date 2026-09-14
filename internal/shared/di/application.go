package di

import "go.uber.org/fx"

// ApplicationModule exposes application-layer use-case handlers (E06).
func ApplicationModule() fx.Option {
	return fx.Module("application")
}
