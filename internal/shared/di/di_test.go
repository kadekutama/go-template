package di

import (
	"testing"

	"go.uber.org/fx"
)

// TestGraphValidates proves the composed fx graph has no dependency cycles.
// Every binary module must appear here; a cycle fails the test before any
// binary is built.
func TestGraphValidates(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		DomainModule(),
		ApplicationModule(),
		InfrastructureModule(),
		RestModule(),
		GrpcModule(),
		GraphQLModule(),
		CronModule(),
		ConsumerModule(),
		fx.NopLogger,
	)
	if err != nil {
		t.Fatalf("fx graph does not validate: %v", err)
	}
}
