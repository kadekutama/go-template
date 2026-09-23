package testcontainers

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// UnleashImage is the pinned Unleash server image for integration suites.
const UnleashImage = "unleashorg/unleash-server:6.5.1"

// MinioImage is the pinned MinIO image for integration suites.
const MinioImage = "minio/minio:RELEASE.2024-12-13T22-19-12Z"

// MaildevImage is the pinned Maildev image for integration suites.
const MaildevImage = "maildev/maildev:2.1.0"

// ServiceHandle is one isolated generic service container.
type ServiceHandle struct {
	container tc.Container
	host      string
	port      string
}

// startService boots one generic container and waits for its TCP port.
func startService(t *testing.T, image, port string, env map[string]string, cmd []string) (*ServiceHandle, error) {
	t.Helper()
	SkipIfNoDocker(t)

	ctx, cancel := Background()
	defer cancel()

	req := tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{port + "/tcp"},
			Env:          env,
			Cmd:          cmd,
			WaitingFor:   wait.ForListeningPort(port + "/tcp"),
		},
		Started: true,
	}

	container, err := tc.GenericContainer(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start %s container: %w", image, err)
	}

	h := &ServiceHandle{container: container}
	t.Cleanup(func() {
		_ = h.Terminate(context.Background())
	})

	mapped, err := container.MappedPort(ctx, port)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("map %s port: %w", image, err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		_ = h.Terminate(context.Background())
		return nil, fmt.Errorf("%s host: %w", image, err)
	}

	h.host = host
	h.port = mapped.Port()

	return h, nil
}

// StartUnleash boots one isolated Unleash server.
func StartUnleash(t *testing.T) (*ServiceHandle, error) {
	t.Helper()

	return startService(t, UnleashImage, "4242", map[string]string{ //nolint:gosec // E07-T09: non-production test placeholder.
		"DATABASE_URL": "postgres://unleash:unleash@localhost/unleash?sslmode=disable",
	}, nil)
}

// StartMinio boots one isolated MinIO object store.
func StartMinio(t *testing.T) (*ServiceHandle, error) {
	t.Helper()

	return startService(t, MinioImage, "9000", map[string]string{
		"MINIO_ROOT_USER":     "minioadmin",
		"MINIO_ROOT_PASSWORD": "minioadmin123",
	}, []string{"server", "/data"})
}

// StartMaildev boots one isolated Maildev SMTP sink.
func StartMaildev(t *testing.T) (*ServiceHandle, error) {
	t.Helper()

	return startService(t, MaildevImage, "1025", nil, nil)
}

// Addr returns the host:port address of the container.
func (h *ServiceHandle) Addr() string {
	return h.host + ":" + h.port
}

// Terminate stops the container; double-terminate returns nil.
func (h *ServiceHandle) Terminate(ctx context.Context) error {
	if h == nil || h.container == nil {
		return nil
	}

	c := h.container
	h.container = nil

	if err := c.Terminate(ctx); err != nil {
		return fmt.Errorf("terminate service container: %w", err)
	}

	return nil
}
