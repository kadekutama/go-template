package log_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

func TestFromContext(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		expectedFields map[string]string
	}

	testCases := []testCase{
		{
			name:           "empty background context",
			ctx:            context.Background(),
			expectedFields: map[string]string{},
		},
		{
			name:           "nil context returns empty map",
			ctx:            nil,
			expectedFields: map[string]string{},
		},
		{
			name: "context with correlation IDs",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = log.WithRequestID(ctx, "req-123")
				ctx = log.WithTraceID(ctx, "trace-456")
				ctx = log.WithTenantID(ctx, "tenant-789")
				ctx = log.WithUserID(ctx, "user-001")
				return ctx
			}(),
			expectedFields: map[string]string{
				log.FieldRequestID: "req-123",
				log.FieldTraceID:   "trace-456",
				log.FieldTenantID:  "tenant-789",
				log.FieldUserID:    "user-001",
			},
		},
		{
			name: "context with overrides merged",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = log.WithRequestID(ctx, "req-initial")
				ctx = log.WithTraceID(ctx, "trace-456")
				return log.WithContext(ctx, map[string]string{
					log.FieldRequestID: "req-overridden",
					"custom_key":       "custom_val",
				})
			}(),
			expectedFields: map[string]string{
				log.FieldRequestID: "req-overridden",
				log.FieldTraceID:   "trace-456",
				"custom_key":       "custom_val",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := log.FromContext(tc.ctx)
			assert.NotNil(t, got)
			assert.Equal(t, tc.expectedFields, got)
		})
	}
}

func TestWithContext(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		fields        map[string]string
		expectedKey   string
		expectedValue string
	}

	testCases := []testCase{
		{
			name:          "nil context creates valid context with fields",
			ctx:           nil,
			fields:        map[string]string{log.FieldRequestID: "req-nil"},
			expectedKey:   log.FieldRequestID,
			expectedValue: "req-nil",
		},
		{
			name:          "existing context receives fields",
			ctx:           context.Background(),
			fields:        map[string]string{"custom_key": "custom_val"},
			expectedKey:   "custom_key",
			expectedValue: "custom_val",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resCtx := log.WithContext(tc.ctx, tc.fields)
			assert.NotNil(t, resCtx)
			extracted := log.FromContext(resCtx)
			assert.Equal(t, tc.expectedValue, extracted[tc.expectedKey])
		})
	}
}

func TestFromContextImmutability(t *testing.T) {
	t.Parallel()

	ctx := log.WithRequestID(context.Background(), "req-orig")
	extracted := log.FromContext(ctx)
	extracted["injected"] = "mutated"

	fresh := log.FromContext(ctx)
	assert.NotContains(t, fresh, "injected")
	assert.Equal(t, "req-orig", fresh[log.FieldRequestID])
}
