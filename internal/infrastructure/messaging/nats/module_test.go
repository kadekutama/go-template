package nats_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	edgenats "github.com/kadekutama/go-template/internal/infrastructure/messaging/nats"
)

// TestModule proves the fx module wires the publisher from Params and that
// the container validates without cycles. ValidateApp never invokes
// constructors, so this stays hermetic despite constructor dialing.
func TestModule(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		edgenats.Module(),
		fx.Provide(func() edgenats.PublisherParams {
			return edgenats.PublisherParams{URL: "nats://127.0.0.1:4222"}
		}),
	)
	require.NoError(t, err)
}
