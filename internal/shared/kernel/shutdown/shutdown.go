// Package shutdown offers one graceful-drain helper shared by every binary
// (E01-T06, SPEC §9.6). serve blocks until ctx ends; then drain runs with a
// timeout before Run returns.
package shutdown

import (
	"context"
	"os/signal"
	"syscall"
	"time"
)

// DefaultDrainTimeout bounds connection draining when callers pass no timeout.
const DefaultDrainTimeout = 10 * time.Second

// Run blocks in serve until ctx is done or SIGTERM/SIGINT arrives, then runs
// drain with timeout. A nil timeout selects DefaultDrainTimeout. The drain
// error (if any) is returned; serve errors are returned immediately.
func Run(ctx context.Context, timeout *time.Duration, serve func(ctx context.Context) error, drain func(ctx context.Context) error) error {
	drainTimeout := DefaultDrainTimeout
	if timeout != nil {
		drainTimeout = *timeout
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	serveErr := make(chan error, 1)
	go func() { serveErr <- serve(ctx) }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- drain(drainCtx) }()

	select {
	case err := <-done:
		return err
	case <-drainCtx.Done():
		return drainCtx.Err()
	}
}
