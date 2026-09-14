package safe

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestGoRecoversPanicAndReports(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reported := make(chan Panic, 1)
	Go(ctx, func(p Panic) { reported <- p }, func(context.Context) {
		panic("boom-test")
	})

	select {
	case p := <-reported:
		if p.Value != "boom-test" {
			t.Errorf("unexpected panic value: %v", p.Value)
		}
		if !strings.Contains(string(p.Stack), "boom-test") && len(p.Stack) == 0 {
			t.Error("expected non-empty stack trace")
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for panic report")
	}
}

func TestGoSilentOnSuccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	reported := make(chan Panic, 1)
	done := make(chan struct{})
	Go(ctx, func(p Panic) { reported <- p }, func(context.Context) {
		close(done)
	})

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timed out waiting for func return")
	}

	select {
	case p := <-reported:
		t.Fatalf("no panic occurred but onPanic fired: %v", p)
	case <-time.After(50 * time.Millisecond):
	}
}
