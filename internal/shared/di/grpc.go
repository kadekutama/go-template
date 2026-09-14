package di

import "go.uber.org/fx"

// GrpcModule holds gRPC-binary providers (services land in E12). See RestModule
// for why shared modules are composed in main, not nested here.
func GrpcModule() fx.Option {
	return fx.Module("grpc")
}
