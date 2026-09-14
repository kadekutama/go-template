// Package trace is the backend-free tracing port (E01-T05, SPEC §7.10).
//
// Middleware and handlers import ONLY this package plus the OTel API
// (trace/propagation contracts) — never the SDK or exporters, which live in
// internal/infrastructure/tracing. Transport-specific carriers (gRPC metadata,
// NATS headers) compose the same W3C propagator in E12/E14.
package trace

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// Tracer starts spans. Implementations wrap an oteltrace.Tracer.
type Tracer interface {
	Start(ctx context.Context, name string, opts ...oteltrace.SpanStartOption) (context.Context, Span)
}

// Span is the consumer-owned span contract.
type Span interface {
	// End finishes the span.
	End(opts ...oteltrace.SpanEndOption)
	// SetAttributes attaches key-value pairs.
	SetAttributes(attrs ...attribute.KeyValue)
	// RecordError marks the span failed (status Error + exception event).
	// The span carries its own context (upstream OTel v1.46 shape).
	RecordError(err error, opts ...oteltrace.EventOption)
	// SpanContext exposes trace/span IDs for log correlation.
	SpanContext() oteltrace.SpanContext
}

// Propagator is the W3C TraceContext + Baggage composite every carrier uses.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}
