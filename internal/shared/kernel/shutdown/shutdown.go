// Package shutdown offers one graceful-drain helper shared by every binary
// (E01-T06, SPEC §9.6). serve blocks until ctx ends; then drain runs with a
// timeout before Run returns.
package shutdown

import (
	"context"
	"errors"
	"os/signal"
	"syscall"
	"time"
)

// Run blocks in serve until ctx is done or SIGTERM/SIGINT arrives, then runs
// drain with timeout. timeout must be positive.
// The drain error (if any) is returned; serve errors are returned immediately.
func Run(ctx context.Context, timeout time.Duration, serve func(ctx context.Context) error, drain func(ctx context.Context) error) error {
	if serve == nil {
		return errors.New("shutdown: serve func must not be nil")
	}
	if timeout <= 0 {
		return errors.New("shutdown: drain timeout must be positive")
	}
	if drain == nil {
		drain = func(context.Context) error { return nil }
	}
	drainTimeout := timeout

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
