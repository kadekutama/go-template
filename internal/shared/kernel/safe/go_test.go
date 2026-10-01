package safe

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

type errCall struct {
	msg  string
	args []any
}

type testLogger struct {
	errChan  chan struct{}
	errCalls atomic.Pointer[[]errCall]
}

func (l *testLogger) Trace(context.Context, string, ...any) {}
func (l *testLogger) Debug(context.Context, string, ...any) {}
func (l *testLogger) Info(context.Context, string, ...any)  {}
func (l *testLogger) Warn(context.Context, string, ...any)  {}
func (l *testLogger) Error(_ context.Context, msg string, args ...any) {
	call := errCall{msg: msg, args: args}
	for {
		old := l.errCalls.Load()
		var next []errCall
		if old != nil {
			next = make([]errCall, len(*old)+1)
			copy(next, *old)
			next[len(*old)] = call
		} else {
			next = []errCall{call}
		}
		if l.errCalls.CompareAndSwap(old, &next) {
			break
		}
	}
	if l.errChan != nil {
		select {
		case l.errChan <- struct{}{}:
		default:
		}
	}
}
func (l *testLogger) calls() []errCall {
	old := l.errCalls.Load()
	if old == nil {
		return nil
	}
	out := make([]errCall, len(*old))
	copy(out, *old)
	return out
}
func (l *testLogger) Panic(_ context.Context, msg string, _ ...any) { panic(msg) }
func (l *testLogger) Fatal(context.Context, string, ...any)         {}
func (l *testLogger) With(...any) log.Logger                        { return l }

func assertPanicLogged(t *testing.T, logger *testLogger) {
	t.Helper()
	calls := logger.calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 error log, got %d", len(calls))
	}
	call := calls[0]
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

	calls := logger.calls()
	if len(calls) != 0 {
		t.Fatalf("expected 0 error logs on success, got %d", len(calls))
	}
}

func TestPanicError(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		p              Panic
		expectedResult string
	}

	testCases := []testCase{
		{
			name: "string panic value",
			p: Panic{
				Value: "something went wrong",
			},
			expectedResult: "safe: recovered panic: something went wrong",
		},
		{
			name: "integer panic value",
			p: Panic{
				Value: 404,
			},
			expectedResult: "safe: recovered panic: 404",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, tc.p.Error())
		})
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

	calls := logger.calls()
	if len(calls) != 1 {
		t.Fatalf("expected panic to be logged even when onPanic is nil, got %d calls", len(calls))
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
