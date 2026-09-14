package trace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/propagation"
)

// MapCarrier adapts a plain string map to a propagation carrier. Transports
// whose headers are string maps (or trivially convertible) use it directly;
// gRPC metadata and NATS headers get thin owned wrappers in E12/E14.
type MapCarrier map[string]string

// Get implements propagation.TextMapCarrier.
func (c MapCarrier) Get(key string) string { return c[key] }

// Set implements propagation.TextMapCarrier.
func (c MapCarrier) Set(key, value string) { c[key] = value }

// Keys implements propagation.TextMapCarrier.
func (c MapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

// Inject writes the context's span context into carrier.
func Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	Propagator().Inject(ctx, carrier)
}

// Extract returns a context carrying the remote span context from carrier.
func Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return Propagator().Extract(ctx, carrier)
}

// headerCarrier adapts net/http headers (canonical MIME keys) to propagation.
type headerCarrier struct{ header http.Header }

func (c headerCarrier) Get(key string) string { return c.header.Get(key) }
func (c headerCarrier) Set(key, value string) { c.header.Set(key, value) }
func (c headerCarrier) Keys() []string {
	keys := make([]string, 0, len(c.header))
	for key := range c.header {
		keys = append(keys, key)
	}
	return keys
}

// InjectHTTP writes trace context into HTTP headers (E11 middleware).
func InjectHTTP(ctx context.Context, header http.Header) {
	Inject(ctx, headerCarrier{header: header})
}

// ExtractHTTP reads trace context from HTTP headers (E11 middleware).
func ExtractHTTP(ctx context.Context, header http.Header) context.Context {
	return Extract(ctx, headerCarrier{header: header})
}
