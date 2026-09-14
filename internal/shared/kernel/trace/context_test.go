package trace

import (
	"context"
	"net/http"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"
)

func TestMapCarrier(t *testing.T) {
	t.Parallel()

	c := MapCarrier{}
	c.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	c.Set("custom", "val")

	type testCase struct {
		name          string
		key           string
		expectedValue string
	}

	testCases := []testCase{
		{
			name:          "existing traceparent key",
			key:           "traceparent",
			expectedValue: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		},
		{
			name:          "custom key",
			key:           "custom",
			expectedValue: "val",
		},
		{
			name:          "nonexistent key",
			key:           "nonexistent",
			expectedValue: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedValue, c.Get(tc.key))
		})
	}

	keys := c.Keys()
	sort.Strings(keys)
	assert.Equal(t, []string{"custom", "traceparent"}, keys)
}

func TestHeaderCarrier(t *testing.T) {
	t.Parallel()

	h := make(http.Header)
	c := headerCarrier{header: h}
	c.Set("x-custom-key", "custom-val")

	type testCase struct {
		name          string
		key           string
		expectedValue string
	}

	testCases := []testCase{
		{
			name:          "existing custom header",
			key:           "x-custom-key",
			expectedValue: "custom-val",
		},
		{
			name:          "nonexistent header",
			key:           "nonexistent",
			expectedValue: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedValue, c.Get(tc.key))
		})
	}

	keys := c.Keys()
	assert.Equal(t, []string{"X-Custom-Key"}, keys)
}

func TestInjectAndExtractHTTP(t *testing.T) {
	traceID, err := oteltrace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	require.NoError(t, err)
	spanID, err := oteltrace.SpanIDFromHex("00f067aa0ba902b7")
	require.NoError(t, err)

	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     true,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	header := make(http.Header)
	InjectHTTP(ctx, header)

	assert.NotEmpty(t, header.Get("traceparent"))

	extractedCtx := ExtractHTTP(context.Background(), header)
	extractedSC := oteltrace.SpanContextFromContext(extractedCtx)

	assert.True(t, extractedSC.IsValid())
	assert.Equal(t, traceID, extractedSC.TraceID())
	assert.Equal(t, spanID, extractedSC.SpanID())
	assert.True(t, extractedSC.IsSampled())
}

func TestInjectAndExtractMapCarrier(t *testing.T) {
	traceID, err := oteltrace.TraceIDFromHex("1234567890abcdef1234567890abcdef")
	require.NoError(t, err)
	spanID, err := oteltrace.SpanIDFromHex("abcdef1234567890")
	require.NoError(t, err)

	sc := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled,
		Remote:     false,
	})
	ctx := oteltrace.ContextWithSpanContext(context.Background(), sc)

	carrier := MapCarrier{}
	Inject(ctx, carrier)

	assert.NotEmpty(t, carrier.Get("traceparent"))

	extractedCtx := Extract(context.Background(), carrier)
	extractedSC := oteltrace.SpanContextFromContext(extractedCtx)

	assert.True(t, extractedSC.IsValid())
	assert.Equal(t, traceID, extractedSC.TraceID())
	assert.Equal(t, spanID, extractedSC.SpanID())
}

func TestPropagator(t *testing.T) {
	prop := Propagator()
	require.NotNil(t, prop)
	fields := prop.Fields()
	assert.Contains(t, fields, "traceparent")
	assert.Contains(t, fields, "baggage")
}

func TestNilSafety(t *testing.T) {
	var nilCtx context.Context
	// Inject and Extract with nil context / carrier / header must not panic
	Inject(nilCtx, nil)
	Inject(context.Background(), nil)
	Inject(nilCtx, MapCarrier{})

	ctx := Extract(nilCtx, nil)
	assert.NotNil(t, ctx)

	ctx = Extract(context.Background(), nil)
	assert.NotNil(t, ctx)

	InjectHTTP(nilCtx, nil)
	InjectHTTP(context.Background(), nil)

	ctx = ExtractHTTP(nilCtx, nil)
	assert.NotNil(t, ctx)

	ctx = ExtractHTTP(context.Background(), nil)
	assert.NotNil(t, ctx)
}
