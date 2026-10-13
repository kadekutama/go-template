package log

import (
	"context"
	"maps"
)

type contextKey struct{}

// fields stored on the context; With merges, FromContext reads.
type contextFields struct {
	values map[string]string
}

// WithContext attaches correlation fields to ctx (merging with existing ones).
func WithContext(ctx context.Context, fields map[string]string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	merged := map[string]string{}

	if existing, ok := ctx.Value(contextKey{}).(contextFields); ok {
		maps.Copy(merged, existing.values)
	}

	maps.Copy(merged, fields)

	return context.WithValue(ctx, contextKey{}, contextFields{values: merged})
}

// FromContext returns the correlation fields on ctx (empty map when absent).
func FromContext(ctx context.Context) map[string]string {
	if ctx == nil {
		return map[string]string{}
	}

	if existing, ok := ctx.Value(contextKey{}).(contextFields); ok {
		out := make(map[string]string, len(existing.values))
		maps.Copy(out, existing.values)

		return out
	}

	return map[string]string{}
}

// Convenience constructors for the four standard fields.

// WithRequestID attaches a request/correlation ID.
func WithRequestID(ctx context.Context, id string) context.Context {
	return WithContext(ctx, map[string]string{FieldRequestID: id})
}

// WithTraceID attaches the distributed trace ID.
func WithTraceID(ctx context.Context, id string) context.Context {
	return WithContext(ctx, map[string]string{FieldTraceID: id})
}

// WithTenantID attaches the tenant scope.
func WithTenantID(ctx context.Context, id string) context.Context {
	return WithContext(ctx, map[string]string{FieldTenantID: id})
}

// WithUserID attaches the acting user.
func WithUserID(ctx context.Context, id string) context.Context {
	return WithContext(ctx, map[string]string{FieldUserID: id})
}
