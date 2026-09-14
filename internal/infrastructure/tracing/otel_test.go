package tracing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

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

func TestBootstrapValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		cfg           Config
		expectedError bool
	}

	testCases := []testCase{
		{
			name: "missing service name",
			cfg: Config{
				ServiceVersion: testServiceVersion,
				Env:            testEnv,
			},
			expectedError: true,
		},
		{
			name: "missing service version",
			cfg: Config{
				ServiceName: testServiceName,
				Env:         testEnv,
			},
			expectedError: true,
		},
		{
			name: "sample ratio greater than 1",
			cfg: Config{
				ServiceName:    testServiceName,
				ServiceVersion: testServiceVersion,
				Env:            testEnv,
				SampleRatio:    9,
			},
			expectedError: true,
		},
		{
			name: "sample ratio less than 0",
			cfg: Config{
				ServiceName:    testServiceName,
				ServiceVersion: testServiceVersion,
				Env:            testEnv,
				SampleRatio:    -0.5,
			},
			expectedError: true,
		},
		{
			name: "bad otlp endpoint",
			cfg: Config{
				ServiceName:    testServiceName,
				ServiceVersion: testServiceVersion,
				Env:            testEnv,
				OTLPEndpoint:   "://bad",
			},
			expectedError: true,
		},
		{
			name: "production requires endpoint",
			cfg: Config{
				ServiceName:    testServiceName,
				ServiceVersion: testServiceVersion,
				Env:            "production",
			},
			expectedError: true,
		},
		{
			name: "valid local without endpoint",
			cfg: Config{
				ServiceName:    testServiceName,
				ServiceVersion: testServiceVersion,
				Env:            testEnv,
			},
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			provider, err := Bootstrap(tc.cfg)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, provider.Exporting())
				_ = provider.Shutdown(context.Background())
			}
		})
	}
}
