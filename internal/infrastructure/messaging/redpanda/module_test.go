package redpanda_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
)

// TestModule proves the fx module wires the producer from Params and that
// the container validates without cycles. ValidateApp never invokes
// constructors and the producer dials lazily, so this stays hermetic — live
// dialing is proven by the integration suite, never by container startup.
func TestModule(t *testing.T) {
	t.Parallel()

	err := fx.ValidateApp(
		redpanda.Module(),
		fx.Provide(func() redpanda.ProducerParams {
			return redpanda.ProducerParams{Seeds: []string{"127.0.0.1:9092"}}
		}),
	)
	require.NoError(t, err)
}
