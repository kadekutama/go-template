package safe

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

type testLogger struct {
	mu       sync.Mutex
	errChan  chan struct{}
	errCalls []struct {
		msg  string
		args []any
	}
}

func (l *testLogger) Trace(context.Context, string, ...any) {}
func (l *testLogger) Debug(context.Context, string, ...any) {}
func (l *testLogger) Info(context.Context, string, ...any)  {}
func (l *testLogger) Warn(context.Context, string, ...any)  {}
func (l *testLogger) Error(_ context.Context, msg string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errCalls = append(l.errCalls, struct {
		msg  string
		args []any
	}{msg: msg, args: args})
	if l.errChan != nil {
		select {
		case l.errChan <- struct{}{}:
		default:
		}
	}
}
func (l *testLogger) Panic(_ context.Context, msg string, _ ...any) { panic(msg) }
func (l *testLogger) Fatal(context.Context, string, ...any)         {}
func (l *testLogger) With(...any) log.Logger                        { return l }

func assertPanicLogged(t *testing.T, logger *testLogger) {
	t.Helper()
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if len(logger.errCalls) != 1 {
		t.Fatalf("expected 1 error log, got %d", len(logger.errCalls))
	}
	call := logger.errCalls[0]
	if call.msg != "recovered panic in goroutine" {
		t.Errorf("unexpected log msg: %s", call.msg)
	}
	hasErrField, hasMetadataField := false, false
	for _, arg := range call.args {
		if f, ok := arg.(log.Field); ok {
			if f.Key == log.FieldError {
				hasErrField = true
			}
			if f.Key == log.FieldMetadata {
				hasMetadataField = true
			}
		}
	}
	if !hasErrField || !hasMetadataField {
		t.Errorf("expected FieldError and FieldMetadata in log args, got %v", call.args)
	}
}

func TestGoRecoversPanicAndReports(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := &testLogger{}
	reported := make(chan Panic, 1)
	Go(ctx, logger, func(p Panic) { reported <- p }, func(context.Context) {
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

	assertPanicLogged(t, logger)
}

func TestGoSilentOnSuccess(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	logger := &testLogger{}
	reported := make(chan Panic, 1)
	done := make(chan struct{})
	Go(ctx, logger, func(p Panic) { reported <- p }, func(context.Context) {
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

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if len(logger.errCalls) != 0 {
		t.Fatalf("expected 0 error logs on success, got %d", len(logger.errCalls))
	}
}

func TestPanicError(t *testing.T) {
	t.Parallel()

	p := Panic{Value: "something went wrong"}
	expected := "safe: recovered panic: something went wrong"
	if p.Error() != expected {
		t.Errorf("got %q, want %q", p.Error(), expected)
	}
}

func TestGoWithNilOnPanic(t *testing.T) {
	t.Parallel()

	logged := make(chan struct{}, 1)
	logger := &testLogger{errChan: logged}
	Go(context.Background(), logger, nil, func(ctx context.Context) {
		panic("panic without handler")
	})

	select {
	case <-logged:
		// Succeeded in recovering and logging without crashing the test
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for panic log")
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()
	if len(logger.errCalls) != 1 {
		t.Fatalf("expected panic to be logged even when onPanic is nil, got %d calls", len(logger.errCalls))
	}
}

func TestGoWithNilLogger(t *testing.T) {
	t.Parallel()

	reported := make(chan Panic, 1)
	Go(context.Background(), nil, func(p Panic) { reported <- p }, func(context.Context) {
		panic("panic without logger")
	})

	select {
	case p := <-reported:
		if p.Value != "panic without logger" {
			t.Errorf("unexpected panic value: %v", p.Value)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for goroutine")
	}
}

func TestGoWithNilFn(t *testing.T) {
	t.Parallel()

	logger := &testLogger{}
	called := false
	Go(context.Background(), logger, func(Panic) { called = true }, nil)
	if called {
		t.Error("onPanic should not be called when fn is nil")
	}
}
