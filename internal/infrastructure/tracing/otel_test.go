package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/kadekutama/go-template/internal/shared/kernel/trace"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// recorderProvider builds a Provider whose spans land in recorder (white-box:
// Bootstrap's exporter path is covered separately).
func recorderProvider() (*Provider, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return &Provider{tracerProvider: tp}, recorder
}

// TestPropagateKeepsTraceIDStrict asserts the exact requirement: the extracted
// trace ID equals the injected one.
func TestPropagateKeepsTraceIDStrict(t *testing.T) {
	t.Parallel()

	provider, _ := recorderProvider()
	tracer := provider.Tracer("test")

	ctx, span := tracer.Start(context.Background(), "op")
	want := span.SpanContext().TraceID()
	carrier := trace.MapCarrier{}
	trace.Inject(ctx, carrier)
	span.End()

	extracted := trace.Extract(context.Background(), carrier)
	_, child := tracer.Start(extracted, "child")
	defer child.End()
	if got := child.SpanContext().TraceID(); got != want {
		t.Errorf("trace ID changed across propagate: %s != %s", got, want)
	}
}

func TestRecordErrorMarksSpan(t *testing.T) {
	t.Parallel()

	provider, recorder := recorderProvider()
	tracer := provider.Tracer("test")

	_, span := tracer.Start(context.Background(), "op")
	span.RecordError(errors.New("boom-test"))
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	if ended[0].Status().Code != codes.Error {
		t.Errorf("expected Error status, got %v", ended[0].Status())
	}
	if len(ended[0].Events()) == 0 {
		t.Error("expected exception event on RecordError")
	}
}

func TestSpanSetAttributes(t *testing.T) {
	t.Parallel()

	provider, recorder := recorderProvider()
	tracer := provider.Tracer("test")

	_, span := tracer.Start(context.Background(), "op")
	span.SetAttributes(attribute.String("test.key", "test.value"), attribute.Int("num", 42))
	span.End()

	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("expected 1 span, got %d", len(ended))
	}
	attrs := ended[0].Attributes()
	foundKey := false
	for _, attr := range attrs {
		if attr.Key == "test.key" && attr.Value.AsString() == "test.value" {
			foundKey = true
			break
		}
	}
	if !foundKey {
		t.Errorf("expected test.key attribute not found in %v", attrs)
	}
}

// testEnv is the non-production environment used across bootstrap tests.
const (
	testServiceName    = "svc"
	testServiceVersion = "v0"
	testEnv            = "local"
)

func TestBootstrapRejectsBadEndpoint(t *testing.T) {
	t.Parallel()

	_, err := Bootstrap(Config{ServiceName: testServiceName, ServiceVersion: testServiceVersion, Env: testEnv, OTLPEndpoint: "://bad"})
	if err == nil {
		t.Fatal("expected endpoint error, got nil")
	}
}

func TestBootstrapRequiresEndpointInProduction(t *testing.T) {
	t.Parallel()

	_, err := Bootstrap(Config{ServiceName: testServiceName, ServiceVersion: testServiceVersion, Env: "production"})
	if err == nil {
		t.Fatal("expected production endpoint error, got nil")
	}
}

func TestBootstrapLocalWithoutEndpoint(t *testing.T) {
	t.Parallel()

	provider, err := Bootstrap(Config{ServiceName: testServiceName, ServiceVersion: testServiceVersion, Env: testEnv})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	if provider.Exporting() {
		t.Error("local bootstrap without endpoint must report exporting=false")
	}
	ctx, span := provider.Tracer("test").Start(context.Background(), "op")
	span.End()
	_ = ctx
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func TestBootstrapRequiresServiceIdentity(t *testing.T) {
	t.Parallel()

	if _, err := Bootstrap(Config{ServiceVersion: testServiceVersion, Env: testEnv}); err == nil {
		t.Error("expected ServiceName error, got nil")
	}
	if _, err := Bootstrap(Config{ServiceName: testServiceName, Env: testEnv}); err == nil {
		t.Error("expected ServiceVersion error, got nil")
	}
	if _, err := Bootstrap(Config{ServiceName: testServiceName, ServiceVersion: testServiceVersion, Env: testEnv, SampleRatio: 9}); err == nil {
		t.Error("expected SampleRatio error for ratio > 1, got nil")
	}
	if _, err := Bootstrap(Config{ServiceName: testServiceName, ServiceVersion: testServiceVersion, Env: testEnv, SampleRatio: -0.5}); err == nil {
		t.Error("expected SampleRatio error for ratio < 0, got nil")
	}
}
