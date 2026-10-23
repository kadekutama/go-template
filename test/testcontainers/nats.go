package testcontainers

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NATSImage is the pinned NATS image for integration suites.
const NATSImage = "nats:2.14.6"

const (
	natsPort         = "4222"
	natsPortTCP      = natsPort + "/tcp"
	natsReadyTimeout = 10 * time.Second
	natsProbeTimeout = 500 * time.Millisecond
)

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
		Image:        NATSImage,
		ExposedPorts: []string{natsPortTCP},
		Cmd:          []string{"-js"},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server is ready"),
			wait.ForListeningPort(natsPortTCP),
		),
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

	mapped, err := container.MappedPort(ctx, natsPort)
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

	if err := waitForNATSConnection(ctx, h.url); err != nil {
		_ = h.Terminate(context.Background())

		return nil, fmt.Errorf("wait for nats client readiness at %s: %w", h.url, err)
	}

	return h, nil
}

// waitForNATSConnection proves the host-mapped endpoint accepts a real NATS
// client handshake before callers receive the handle.
func waitForNATSConnection(ctx context.Context, url string) error {
	waitCtx, cancel := context.WithTimeout(ctx, natsReadyTimeout)
	defer cancel()

	var lastErr error

	for {
		conn, err := natsgo.Connect(url, natsgo.Timeout(natsProbeTimeout))
		if err == nil {
			conn.Close()

			return nil
		}

		lastErr = err

		select {
		case <-waitCtx.Done():
			return fmt.Errorf("nats did not accept a client connection: %w", errors.Join(waitCtx.Err(), lastErr))
		case <-time.After(time.Duration(pollIntervalMs) * time.Millisecond):
		}
	}
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
