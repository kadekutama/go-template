package log

import "context"

type contextKey struct{}

// fields stored on the context; With merges, FromContext reads.
type contextFields struct {
	values map[string]string
}

// WithContext attaches correlation fields to ctx (merging with existing ones).
func WithContext(ctx context.Context, fields map[string]string) context.Context {
	merged := map[string]string{}
	if existing, ok := ctx.Value(contextKey{}).(contextFields); ok {
		for key, val := range existing.values {
			merged[key] = val
		}
	}
	for key, val := range fields {
		merged[key] = val
	}
	return context.WithValue(ctx, contextKey{}, contextFields{values: merged})
}

// FromContext returns the correlation fields on ctx (empty map when absent).
func FromContext(ctx context.Context) map[string]string {
	if existing, ok := ctx.Value(contextKey{}).(contextFields); ok {
		out := make(map[string]string, len(existing.values))
		for key, val := range existing.values {
			out[key] = val
		}
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
