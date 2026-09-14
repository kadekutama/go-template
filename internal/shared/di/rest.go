package di

import "go.uber.org/fx"

// RestModule holds REST-binary providers (handlers land in E11). Mains compose
// it with the shared Domain/Application/Infrastructure modules; nesting them
// here would provide constructors twice and fail graph validation.
func RestModule() fx.Option {
	return fx.Module("rest")
}
