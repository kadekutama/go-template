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
