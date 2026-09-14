package di

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	assert.NoError(t, err)
}

func TestProvidersConstructInstances(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		validate func() bool
	}

	testCases := []testCase{
		{
			name: "logger provider returns non-nil",
			validate: func() bool {
				return ProvideLogger() != nil
			},
		},
		{
			name: "clock provider returns valid clock",
			validate: func() bool {
				clk := ProvideClock()
				return clk != nil && !clk.Now().IsZero()
			},
		},
		{
			name: "id generator provider returns valid generator",
			validate: func() bool {
				gen := ProvideIDGenerator()
				return gen != nil && gen.NewID() != ""
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.True(t, tc.validate())
		})
	}
}
