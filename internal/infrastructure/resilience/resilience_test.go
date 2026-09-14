package resilience

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	kernel "example.com/go-template/internal/shared/kernel/resilience"
)

// fakeSleep records waits without wall-clock delays and honors cancellation.
type fakeSleep struct {
	waits []time.Duration
}

func (f *fakeSleep) sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.waits = append(f.waits, d)
	return nil
}

func TestBreakerOpensAndRecovers(t *testing.T) {
	t.Parallel()

	var transitions []string
	breaker := New("test", Settings{
		FailureThreshold: 2,
		ProbeTimeout:     30 * time.Millisecond,
		OnStateChange: func(_ string, from, to kernel.State) {
			transitions = append(transitions, fmt.Sprintf("%d-%d", int(from), int(to)))
		},
	})
	ctx := context.Background()
	fail := func(context.Context) error { return &kernel.RetryableError{Err: errors.New("down")} }

	if err := breaker.Execute(ctx, "op", "k", fail); err == nil {
		t.Fatal("expected failure 1")
	}
	if got := breaker.State(); got != kernel.StateClosed {
		t.Fatalf("want closed after 1 failure, got %v", got)
	}
	if err := breaker.Execute(ctx, "op", "k", fail); err == nil {
		t.Fatal("expected failure 2")
	}
	if got := breaker.State(); got != kernel.StateOpen {
		t.Fatalf("want open after threshold, got %v", got)
	}

	called := false
	err := breaker.Execute(ctx, "op", "k", func(context.Context) error { called = true; return nil })
	if called {
		t.Error("open breaker must fast-fail without calling fn")
	}
	if !errors.Is(err, ErrBreakerOpen) {
		t.Errorf("want ErrBreakerOpen, got %v", err)
	}

	time.Sleep(60 * time.Millisecond) // pass the 30ms probe timeout with margin
	if err := breaker.Execute(ctx, "op", "k", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("half-open probe: %v", err)
	}
	if got := breaker.State(); got != kernel.StateClosed {
		t.Errorf("successful probe should close, got %v (transitions %v)", got, transitions)
	}
}

func TestBreakerRespectsCancellation(t *testing.T) {
	t.Parallel()

	breaker := New("test", Settings{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := breaker.Execute(ctx, "op", "k", func(context.Context) error { called = true; return nil }); err == nil {
		t.Fatal("expected context error, got nil")
	}
	if called {
		t.Error("cancelled call must not run fn")
	}
}

func TestRetrySucceedsAfterFailures(t *testing.T) {
	t.Parallel()

	fake := &fakeSleep{}
	calls := 0
	err := Execute(context.Background(), fake.sleep,
		kernel.RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Multiplier: 1},
		false, "key-1", kernel.DefaultClassifier,
		func(context.Context) error {
			calls++
			if calls < 3 {
				return &kernel.RetryableError{Err: errors.New("flaky")}
			}
			return nil
		})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if calls != 3 || len(fake.waits) != 2 {
		t.Errorf("want 3 calls / 2 waits, got %d / %d", calls, len(fake.waits))
	}
}

func TestNoBlindRetryWithoutKey(t *testing.T) {
	t.Parallel()

	fake := &fakeSleep{}
	calls := 0
	err := Execute(context.Background(), fake.sleep,
		kernel.RetryPolicy{MaxAttempts: 5, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Multiplier: 1},
		false, "", kernel.DefaultClassifier,
		func(context.Context) error {
			calls++
			return &kernel.RetryableError{Err: errors.New("down")}
		})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("non-idempotent call without key must run exactly once, ran %d", calls)
	}
	if len(fake.waits) != 0 {
		t.Errorf("no backoff expected, got %v", fake.waits)
	}
}

func TestRetryExhaustion(t *testing.T) {
	t.Parallel()

	fake := &fakeSleep{}
	calls := 0
	err := Execute(context.Background(), fake.sleep,
		kernel.RetryPolicy{MaxAttempts: 3, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond, Multiplier: 1},
		false, "key-1", kernel.DefaultClassifier,
		func(context.Context) error {
			calls++
			return &kernel.RetryableError{Err: errors.New("down")}
		})
	if err == nil || calls != 3 {
		t.Errorf("want exhaustion after 3 calls, got calls=%d err=%v", calls, err)
	}
}

func TestCancelledContextAborts(t *testing.T) {
	t.Parallel()

	fake := &fakeSleep{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	if err := Execute(ctx, fake.sleep,
		kernel.RetryPolicy{MaxAttempts: 3}, false, "key-1", kernel.DefaultClassifier,
		func(context.Context) error { called = true; return nil }); err == nil {
		t.Fatal("expected context error, got nil")
	}
	if called {
		t.Error("cancelled execute must not call fn")
	}
}

func TestPortHasNoGobreakerImport(t *testing.T) {
	t.Parallel()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
	cmd := exec.CommandContext(t.Context(), "go", "list", "-f", `{{join .Imports " "}}`,
		"example.com/go-template/internal/shared/kernel/resilience")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	if strings.Contains(string(out), "sony/gobreaker") {
		t.Errorf("kernel port must not import gobreaker: %s", out)
	}
}
