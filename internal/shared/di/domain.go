// Package di wires each architecture layer as an fx.Option constructor.
//
// Every layer registers exactly one module constructor. Constructors return
// fx.Option values built only from fx.Provide/fx.Invoke; there is no
// package-level state, no init(), and no global registry. Each cmd/* entrypoint
// composes the modules it needs. Later tasks add providers to these
// constructors with a coordination note in their task packet.
package di

import "go.uber.org/fx"

// DomainModule exposes domain-layer constructors (pure domain services land here in E02).
func DomainModule() fx.Option {
	return fx.Module("domain")
}
