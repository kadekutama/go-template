package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"example.com/go-template/internal/shared/kernel/log"
	"example.com/go-template/internal/shared/kernel/safe"
)

const testDebugLevel = "debug"

func TestLineCarriesCorrelationIDs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: testDebugLevel})

	ctx := log.WithRequestID(log.WithTraceID(context.Background(), "trace-1"), "req-1")
	logger.Info(ctx, "hello", "extra", "x")

	out := buf.String()
	for _, want := range []string{`"request_id":"req-1"`, `"trace_id":"trace-1"`, `"msg":"hello"`, `"extra":"x"`} {
		if !strings.Contains(out, want) {
			t.Errorf("log line missing %s: %s", want, out)
		}
	}
}

func TestWithChildKeepsParentClean(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	parent := New(&buf, Config{Level: testDebugLevel})
	child := parent.With("k", "v")
	child.Warn(context.Background(), "child-line")

	lines := strings.TrimSpace(buf.String())
	if !strings.Contains(lines, `"k":"v"`) {
		t.Errorf("child line missing bound attr: %s", lines)
	}

	buf.Reset()
	parent.Warn(context.Background(), "parent-line")
	if strings.Contains(buf.String(), `"k":"v"`) {
		t.Errorf("parent logger leaked child attr: %s", buf.String())
	}
}

func TestLevelFilters(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: "error"})
	logger.Debug(context.Background(), "quiet")
	if buf.Len() != 0 {
		t.Errorf("debug line emitted at error level: %s", buf.String())
	}
	logger.Error(context.Background(), "loud")
	if !strings.Contains(buf.String(), "loud") {
		t.Errorf("error line missing at error level: %s", buf.String())
	}
}

func TestUnknownLevelFallsBackToInfo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: "nonsense"})
	logger.Info(context.Background(), "still-here")
	if !strings.Contains(buf.String(), "still-here") {
		t.Error("unknown level should fail open to info")
	}
}

func TestDiscardSatisfiesPort(t *testing.T) {
	t.Parallel()

	logger := Discard()
	logger.Info(context.Background(), "dropped")
	if logger.With("k", "v") == nil {
		t.Error("Discard().With must return a Logger")
	}
}

func TestTraceFiltering(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	New(&buf, Config{Level: "trace"}).Trace(context.Background(), "deep")
	if !strings.Contains(buf.String(), `"level":"trace"`) {
		t.Errorf("trace line missing at trace level: %s", buf.String())
	}

	buf.Reset()
	New(&buf, Config{Level: "info"}).Trace(context.Background(), "deep")
	if buf.Len() != 0 {
		t.Errorf("trace line emitted at info level: %s", buf.String())
	}
}

func TestPanicEmitsThenPanics(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: testDebugLevel})

	func() {
		defer func() {
			recovered := recover()
			if recovered != "boom-log" {
				t.Errorf("want panic value %q, got %v", "boom-log", recovered)
			}
		}()
		logger.Panic(context.Background(), "boom-log", "k", "v")
	}()

	out := buf.String()
	if !strings.Contains(out, `"level":"panic"`) || !strings.Contains(out, "boom-log") {
		t.Errorf("panic line missing: %s", out)
	}
}

func TestPanicRecoverableViaSafeGo(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: testDebugLevel})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	reported := make(chan safe.Panic, 1)
	safe.Go(ctx, func(p safe.Panic) { reported <- p }, func(context.Context) {
		logger.Panic(context.Background(), "via-safe-go")
	})

	select {
	case p := <-reported:
		if p.Value != "via-safe-go" {
			t.Errorf("unexpected panic value: %v", p.Value)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for panic report")
	}
}

func TestFatalStubsExit(t *testing.T) {
	t.Parallel()

	oldExit := osExit
	defer func() { osExit = oldExit }()

	var buf bytes.Buffer
	var code int
	calls := 0
	osExit = func(c int) { code = c; calls++ }

	New(&buf, Config{Level: testDebugLevel}).Fatal(context.Background(), "no-return", "k", "v")

	out := buf.String()
	if !strings.Contains(out, `"level":"fatal"`) || !strings.Contains(out, "no-return") {
		t.Errorf("fatal line missing: %s", out)
	}
	if calls != 1 || code != 1 {
		t.Errorf("want one exit(1), got calls=%d code=%d", calls, code)
	}
}

func TestOddArgsPreserved(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := New(&buf, Config{Level: testDebugLevel})
	logger.Info(context.Background(), "odd-check", "k1", "v1", "orphan")

	out := buf.String()
	if !strings.Contains(out, `"k1":"v1"`) || !strings.Contains(out, `"!EXTRA_ARG":"orphan"`) {
		t.Errorf("expected odd arg to be preserved under !EXTRA_ARG, got: %s", out)
	}
}
