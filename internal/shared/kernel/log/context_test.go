package log_test

import (
	"context"
	"testing"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestContextCorrelationFields(t *testing.T) {
	t.Parallel()

	// Empty context returns empty map, not nil
	empty := log.FromContext(context.Background())
	if len(empty) != 0 {
		t.Errorf("empty context should have 0 fields, got: %v", empty)
	}

	// Add fields one by one
	ctx := context.Background()
	ctx = log.WithRequestID(ctx, "req-123")
	ctx = log.WithTraceID(ctx, "trace-456")
	ctx = log.WithTenantID(ctx, "tenant-789")
	ctx = log.WithUserID(ctx, "user-001")

	fields := log.FromContext(ctx)
	if fields[log.FieldRequestID] != "req-123" {
		t.Errorf("FieldRequestID = %q, want %q", fields[log.FieldRequestID], "req-123")
	}
	if fields[log.FieldTraceID] != "trace-456" {
		t.Errorf("FieldTraceID = %q, want %q", fields[log.FieldTraceID], "trace-456")
	}
	if fields[log.FieldTenantID] != "tenant-789" {
		t.Errorf("FieldTenantID = %q, want %q", fields[log.FieldTenantID], "tenant-789")
	}
	if fields[log.FieldUserID] != "user-001" {
		t.Errorf("FieldUserID = %q, want %q", fields[log.FieldUserID], "user-001")
	}

	// WithContext merges with existing fields and allows overwriting
	overrides := map[string]string{
		log.FieldRequestID: "req-overridden",
		"custom_key":       "custom_val",
	}
	ctx = log.WithContext(ctx, overrides)
	merged := log.FromContext(ctx)
	if merged[log.FieldRequestID] != "req-overridden" {
		t.Errorf("FieldRequestID = %q, want %q", merged[log.FieldRequestID], "req-overridden")
	}
	if merged[log.FieldTraceID] != "trace-456" {
		t.Errorf("FieldTraceID preserved = %q, want %q", merged[log.FieldTraceID], "trace-456")
	}
	if merged["custom_key"] != "custom_val" {
		t.Errorf("custom_key = %q, want %q", merged["custom_key"], "custom_val")
	}

	// Mutating returned map does not affect context
	merged["injected"] = "mutated"
	fresh := log.FromContext(ctx)
	if _, exists := fresh["injected"]; exists {
		t.Error("FromContext returned map must not leak back into context")
	}
}

func TestNilContextSafety(t *testing.T) {
	t.Parallel()

	var nilCtx context.Context
	// FromContext with nil must return empty map without panicking
	got := log.FromContext(nilCtx)
	if got == nil || len(got) != 0 {
		t.Errorf("FromContext(nil) = %v, want empty non-nil map", got)
	}

	// WithContext with nil must create valid context without panicking
	ctx := log.WithContext(nilCtx, map[string]string{log.FieldRequestID: "req-nil"})
	if ctx == nil {
		t.Fatal("WithContext(nil, ...) returned nil context")
	}
	fields := log.FromContext(ctx)
	if fields[log.FieldRequestID] != "req-nil" {
		t.Errorf("FieldRequestID = %q, want req-nil", fields[log.FieldRequestID])
	}
}
