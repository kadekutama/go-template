// Package tracing bootstraps the OpenTelemetry SDK behind the kernel trace
// port (E01-T05). Middleware in E11/E12/E13 consumes trace.Tracer; the SDK,
// batching, and exporters never leak past this package.
package tracing

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"example.com/go-template/internal/shared/kernel/trace"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Provider is the bootstrapped tracing handle.
type Provider struct {
	tracerProvider *sdktrace.TracerProvider
	exporting      bool
}

// Tracer returns a kernel-port tracer backed by the SDK provider.
func (p *Provider) Tracer(name string) trace.Tracer {
	return otelTracer{inner: p.tracerProvider.Tracer(name)}
}

// Exporting reports whether spans leave the process (OTLP configured).
// Local development without an endpoint is explicit, not silent: callers can
// log or metric on false in non-prod environments.
func (p *Provider) Exporting() bool { return p.exporting }

// Shutdown flushes buffered spans with the caller's context.
func (p *Provider) Shutdown(ctx context.Context) error {
	return p.tracerProvider.Shutdown(ctx)
}

// Bootstrap builds the SDK provider per cfg. It sets the global OTel provider
// and propagator — the single sanctioned exception to no-global-state, per
// upstream OTel guidance (instrumentation libraries read globals). Callers
// preferring explicit threading use Provider.Tracer instead of otel.Tracer.
func Bootstrap(cfg Config) (*Provider, error) {
	if strings.TrimSpace(cfg.ServiceName) == "" {
		return nil, fmt.Errorf("tracing: ServiceName is required")
	}
	if strings.TrimSpace(cfg.ServiceVersion) == "" {
		return nil, fmt.Errorf("tracing: ServiceVersion is required")
	}
	ratio := cfg.SampleRatio
	if ratio <= 0 {
		ratio = DefaultSampleRatio
	}
	if ratio < 0 || ratio > 1 {
		return nil, fmt.Errorf("tracing: SampleRatio %v out of [0,1]", cfg.SampleRatio)
	}

	var processor sdktrace.SpanProcessor
	exporting := false
	if strings.TrimSpace(cfg.OTLPEndpoint) != "" {
		exporter, err := newOTLPExporter(context.Background(), cfg.OTLPEndpoint)
		if err != nil {
			return nil, err
		}
		processor = sdktrace.NewBatchSpanProcessor(exporter)
		exporting = true
	} else if strings.EqualFold(cfg.Env, "production") {
		return nil, fmt.Errorf("tracing: production requires OTLPEndpoint (refusing silent no-op)")
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(resource.NewSchemaless(
			attribute.String("service.name", cfg.ServiceName),
			attribute.String("service.version", cfg.ServiceVersion),
			attribute.String("deployment.environment", cfg.Env),
		)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))),
	}
	if processor != nil {
		opts = append(opts, sdktrace.WithSpanProcessor(processor))
	}
	tp := sdktrace.NewTracerProvider(opts...)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(trace.Propagator())

	return &Provider{tracerProvider: tp, exporting: exporting}, nil
}

// newOTLPExporter validates the endpoint fast (clear error, no dial yet —
// gRPC dials lazily) and builds the exporter. Accepted form:
// https?://host:port (http selects insecure transport).
func newOTLPExporter(ctx context.Context, endpoint string) (*otlptrace.Exporter, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil, fmt.Errorf("tracing: invalid OTLPEndpoint %q (want https?://host:port)", endpoint)
	}
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(parsed.Host)}
	if parsed.Scheme == "http" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("tracing: build OTLP exporter: %w", err)
	}
	return exporter, nil
}

// otelTracer adapts the SDK tracer to the kernel port.
type otelTracer struct {
	inner oteltrace.Tracer
}

// Start implements trace.Tracer.
func (t otelTracer) Start(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := t.inner.Start(ctx, name, opts...)
	return ctx, otelSpan{inner: span}
}

// otelSpan adapts the SDK span to the kernel port.
type otelSpan struct {
	inner oteltrace.Span
}

// End implements trace.Span.
func (s otelSpan) End(opts ...oteltrace.SpanEndOption) { s.inner.End(opts...) }

// SetAttributes implements trace.Span.
func (s otelSpan) SetAttributes(attrs ...attribute.KeyValue) { s.inner.SetAttributes(attrs...) }

// RecordError implements trace.Span: the error is recorded as an exception
// event AND the span status becomes Error (head sampling cannot foresee
// errors, so every error is at least visible; E15 owns tail rules).
func (s otelSpan) RecordError(err error, opts ...oteltrace.EventOption) {
	s.inner.RecordError(err, opts...)
	s.inner.SetStatus(codes.Error, err.Error())
}

// SpanContext implements trace.Span.
func (s otelSpan) SpanContext() oteltrace.SpanContext { return s.inner.SpanContext() }
