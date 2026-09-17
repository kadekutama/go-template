package testcontainers

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NATSImage is the pinned NATS image for integration suites.
const NATSImage = "nats:2.14.6"

// NATSHandle is one isolated NATS container (supports NATS Core edge fanout and JetStream).
type NATSHandle struct {
	container tc.Container
	url       string
}

// StartNATS boots one isolated NATS server (with JetStream enabled for compatibility) and waits
// until it is ready. Cleanup is registered on t; Terminate is idempotent.
func StartNATS(t *testing.T) (*NATSHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := Background()
	defer cancel()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        NATSImage,
			ExposedPorts: []string{"4222/tcp"},
			Cmd:          []string{"-js"},
			WaitingFor:   wait.ForLog("Server is ready"),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start nats container: %w", err)
	}

	h := &NATSHandle{container: container}
	t.Cleanup(func() {
		_ = h.Terminate(context.Background())
	})

	mapped, err := container.MappedPort(ctx, "4222")
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("map nats port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("nats host: %w", err)
	}

	h.url = "nats://" + host + ":" + mapped.Port()

	return h, nil
}

// URL returns the client URL of the container.
func (h *NATSHandle) URL() string {
	return h.url
}

// Terminate stops the container; double-terminate returns nil.
func (h *NATSHandle) Terminate(ctx context.Context) error {
	if h == nil || h.container == nil {
		return nil
	}

	c := h.container
	h.container = nil

	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate nats container: %w", err)
	}

	return nil
}
