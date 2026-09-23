package testcontainers

import (
	"context"
	"fmt"
	"testing"

	"github.com/testcontainers/testcontainers-go/modules/redpanda"
)

// redpandaTestImage pins the container image for suites (the testcontainers
// module default is older; deployments pin the SPEC server version
// separately). The suite proves the Kafka-API data plane, which is
// version-independent across these images.
const redpandaTestImage = "docker.redpanda.com/redpandadata/redpanda:v23.3.3"

type RedpandaHandle struct {
	container *redpanda.Container
	seed      string
}

// StartRedpanda boots one isolated Redpanda container and waits until the
// Kafka API accepts connections. Cleanup is registered on t; Terminate is
// idempotent.
func StartRedpanda(t *testing.T) (*RedpandaHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := Background()
	defer cancel()

	container, err := redpanda.Run(ctx, redpandaTestImage)
	if err != nil {
		return nil, fmt.Errorf("start redpanda container: %w", err)
	}

	h := &RedpandaHandle{container: container}
	t.Cleanup(func() {
		_ = h.Terminate(context.Background())
	})

	seed, err := container.KafkaSeedBroker(ctx)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("redpanda seed broker: %w", err)
	}

	h.seed = seed

	return h, nil
}

// Seed returns the host:port bootstrap address of the container.
func (h *RedpandaHandle) Seed() string {
	return h.seed
}

// Terminate stops the container; double-terminate returns nil.
func (h *RedpandaHandle) Terminate(ctx context.Context) error {
	if h == nil || h.container == nil {
		return nil
	}

	c := h.container
	h.container = nil

	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate redpanda container: %w", err)
	}

	return nil
}
