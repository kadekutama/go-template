package shutdown

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunDrainsOnCancel(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	release := make(chan struct{})
	var drained atomic.Bool

	serve := func(ctx context.Context) error {
		<-ctx.Done()
		<-release // hold until the test frees us; proves Run chose the drain path
		return nil
	}
	drain := func(context.Context) error {
		drained.Store(true)
		return nil
	}

	time.AfterFunc(100*time.Millisecond, cancel)
	timeout := 5 * time.Second
	if err := Run(ctx, &timeout, serve, drain); err != nil {
		t.Fatalf("Run: %v", err)
	}
	close(release)
	if !drained.Load() {
		t.Error("drain did not run after cancel")
	}
}

func TestRunReturnsServeErrorImmediately(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("serve boom")
	err := Run(context.Background(), nil, func(context.Context) error {
		return sentinel
	}, func(context.Context) error {
		t.Error("drain must not run when serve fails first")
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("got %v, want sentinel", err)
	}
}

func TestRunNilServeReturnsError(t *testing.T) {
	t.Parallel()

	err := Run(context.Background(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil serve, got nil")
	}
}

func TestRunNilDrainSucceeds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Run(ctx, nil, func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, nil)
	if err != nil {
		t.Fatalf("expected nil drain to succeed, got %v", err)
	}
}

func TestRunReturnsDrainError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	drainErr := errors.New("drain failure")
	err := Run(ctx, nil, func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, func(context.Context) error {
		return drainErr
	})
	if !errors.Is(err, drainErr) {
		t.Fatalf("got %v, want drainErr %v", err, drainErr)
	}
}

func TestRunReturnsDrainTimeoutError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	timeout := 20 * time.Millisecond
	err := Run(ctx, &timeout, func(ctx context.Context) error {
		<-ctx.Done()
		return nil
	}, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}
