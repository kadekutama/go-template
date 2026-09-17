package testcontainers

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// ValkeyImage is the pinned Valkey image for integration suites.
const ValkeyImage = "valkey/valkey:9.0.6"

// ValkeyHandle is one isolated Valkey container.
type ValkeyHandle struct {
	container tc.Container
	host      string
	port      string
}

// StartValkey boots one isolated Valkey container and waits until it accepts
// connections. Cleanup is registered on t; Terminate is idempotent.
func StartValkey(t *testing.T) (*ValkeyHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := Background()
	defer cancel()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        ValkeyImage,
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForListeningPort("6379/tcp"),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start valkey container: %w", err)
	}

	h := &ValkeyHandle{container: container}
	t.Cleanup(func() {
		_ = h.Terminate(context.Background())
	})

	mapped, err := container.MappedPort(ctx, "6379")
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("map valkey port: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("valkey host: %w", err)
	}

	h.host = host
	h.port = mapped.Port()

	return h, nil
}

// Addr returns the host:port address of the container.
func (h *ValkeyHandle) Addr() string {
	return h.host + ":" + h.port
}

// Terminate stops the container; double-terminate returns nil.
func (h *ValkeyHandle) Terminate(ctx context.Context) error {
	if h == nil || h.container == nil {
		return nil
	}

	c := h.container
	h.container = nil

	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate valkey container: %w", err)
	}

	return nil
}
