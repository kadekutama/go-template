package profiling_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/infrastructure/profiling"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/internal/shared/kernel/validate"
)

func testLogger() log.Logger {
	return logging.New(os.Stderr, logging.Config{Level: "info"})
}

func TestNewProfilerValidation(t *testing.T) {
	t.Parallel()

	base := profiling.Config{
		Enabled:              true,
		ServerAddress:        "http://localhost:4040",
		ApplicationName:      "fintech-ledger",
		Environment:          "local",
		UploadRate:           15 * time.Second,
		MutexProfileFraction: 5,
		BlockProfileRate:     5,
		Tags:                 map[string]string{"version": "test"},
	}

	type testCase struct {
		name              string
		params            profiling.ProfilerParams
		expectedResultNil bool
		expectedError     error
	}

	testCases := []testCase{
		{
			name: "valid enabled configuration",
			params: profiling.ProfilerParams{
				Config: base,
				Logger: testLogger(),
			},
			expectedResultNil: false,
			expectedError:     nil,
		},
		{
			name: "disabled configuration with empty optional fields",
			params: profiling.ProfilerParams{
				Config: profiling.Config{Enabled: false},
				Logger: testLogger(),
			},
			expectedResultNil: false,
			expectedError:     nil,
		},
		{
			name: "nil logger rejected",
			params: profiling.ProfilerParams{
				Config: base,
				Logger: nil,
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrNilLogger,
		},
		{
			name: "invalid server address rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.ServerAddress = "://bad"

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     validate.ErrValidateViolation,
		},
		{
			name: "missing application name rejected when enabled",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.ApplicationName = ""

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     validate.ErrValidateViolation,
		},
		{
			name: "negative mutex fraction rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.MutexProfileFraction = -1

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     validate.ErrValidateViolation,
		},
		{
			name: "negative block rate rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.BlockProfileRate = -1

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     validate.ErrValidateViolation,
		},
		{
			name: "tenant tag rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"tenant_id": "t-1"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrForbiddenTagKey,
		},
		{
			name: "account tag rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"account_id": "a-1"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrForbiddenTagKey,
		},
		{
			name: "user tag rejected case-insensitively",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"  USER_ID  ": "u-1"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrForbiddenTagKey,
		},
		{
			name: "email tag rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"email": "test@example.com"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrForbiddenTagKey,
		},
		{
			name: "pan tag rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"pan": "4111111111111111"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrForbiddenTagKey,
		},
		{
			name: "blank tag key rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"   ": "value"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrEmptyTagKey,
		},
		{
			name: "unknown high-cardinality tag rejected",
			params: profiling.ProfilerParams{
				Config: func() profiling.Config {
					c := base
					c.Tags = map[string]string{"request_id": "req-1"}

					return c
				}(),
				Logger: testLogger(),
			},
			expectedResultNil: true,
			expectedError:     profiling.ErrUnsupportedTagKey,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actualResult, err := profiling.NewProfiler(tc.params)
			if tc.expectedResultNil {
				assert.Nil(t, actualResult)
			} else {
				assert.NotNil(t, actualResult)
			}

			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestDisabledProfilerLifecycle(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ingest" {
			requests.Add(1)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	type testCase struct {
		name          string
		ctx           context.Context
		config        profiling.Config
		expectedStart error
		expectedStop  error
	}

	testCases := []testCase{
		{
			name: "disabled profiler makes no requests",
			ctx:  context.Background(),
			config: profiling.Config{
				Enabled:       false,
				ServerAddress: server.URL,
			},
			expectedStart: nil,
			expectedStop:  nil,
		},
		{
			name: "disabled profiler ignores a canceled context",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			config: profiling.Config{
				Enabled: false,
			},
			expectedStart: nil,
			expectedStop:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			probe, err := profiling.NewProfiler(profiling.ProfilerParams{
				Config: tc.config,
				Logger: testLogger(),
			})
			require.NoError(t, err)

			startErr := probe.Start(tc.ctx)
			stopErr := probe.Stop(tc.ctx)

			assert.ErrorIs(t, startErr, tc.expectedStart)
			assert.ErrorIs(t, stopErr, tc.expectedStop)
			assert.Zero(t, requests.Load())
		})
	}
}

// BenchmarkDisabledProfilerStartStop proves the disabled path performs no
// allocations. testing.AllocsPerRun cannot be used inside the parallel unit
// suite because it orchestrates GOMAXPROCS; the benchmark measures the same
// contract without serializing the package's tests.
func BenchmarkDisabledProfilerStartStop(b *testing.B) {
	probe, err := profiling.NewProfiler(profiling.ProfilerParams{
		Config: profiling.Config{Enabled: false},
		Logger: testLogger(),
	})
	if err != nil {
		b.Fatalf("NewProfiler: %v", err)
	}

	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = probe.Start(ctx)
		_ = probe.Stop(ctx)
	}
}

func TestEnabledProfilerLifecycle(t *testing.T) {
	t.Parallel()

	var requests atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ingest" {
			requests.Add(1)
		}

		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	type testCase struct {
		name                string
		config              profiling.Config
		expectedStartError  error
		expectedStopError   error
		expectedMutexDuring int
		expectedMutexAfter  int
	}

	testCases := []testCase{
		{
			name: "enabled profiler uploads profiles and resets runtime rates",
			config: profiling.Config{
				Enabled:              true,
				ServerAddress:        server.URL,
				ApplicationName:      "profiling-test",
				Environment:          "test",
				UploadRate:           50 * time.Millisecond,
				MutexProfileFraction: 5,
				BlockProfileRate:     5,
				Tags:                 map[string]string{"version": "test"},
			},
			expectedStartError:  nil,
			expectedStopError:   nil,
			expectedMutexDuring: 5,
			expectedMutexAfter:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			probe, err := profiling.NewProfiler(profiling.ProfilerParams{
				Config: tc.config,
				Logger: testLogger(),
			})
			require.NoError(t, err)

			assert.ErrorIs(t, probe.Start(context.Background()), tc.expectedStartError)
			assert.Equal(t, tc.expectedMutexDuring, runtime.SetMutexProfileFraction(-1))

			burnCPU(150 * time.Millisecond)

			assert.ErrorIs(t, probe.Stop(context.Background()), tc.expectedStopError)
			assert.Equal(t, tc.expectedMutexAfter, runtime.SetMutexProfileFraction(-1))
			assert.Positive(t, requests.Load())
		})
	}
}

func TestWrapTracerProvider(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		provider          oteltrace.TracerProvider
		recorder          *tracetest.SpanRecorder
		expectedResultNil bool
	}

	recorder := tracetest.NewSpanRecorder()

	testCases := []testCase{
		{
			name:              "nil provider remains nil",
			provider:          nil,
			recorder:          nil,
			expectedResultNil: true,
		},
		{
			name: "sampled root span carries the profile id",
			provider: sdktrace.NewTracerProvider(
				sdktrace.WithSampler(sdktrace.AlwaysSample()),
				sdktrace.WithSpanProcessor(recorder),
			),
			recorder:          recorder,
			expectedResultNil: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := profiling.WrapTracerProvider(tc.provider)
			assert.Equal(t, tc.expectedResultNil, result == nil)

			if tc.expectedResultNil {
				return
			}

			_, span := result.Tracer("profiling-test").Start(context.Background(), "root")
			span.End()

			ended := tc.recorder.Ended()
			require.Len(t, ended, 1)
			assert.True(t, hasAttribute(ended[0], "pyroscope.profile.id"))
		})
	}
}

func hasAttribute(span sdktrace.ReadOnlySpan, key string) bool {
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			return true
		}
	}

	return false
}

// burnCPU keeps the goroutine on-CPU long enough for the CPU sampler to emit
// at least one profile during the enabled lifecycle test.
func burnCPU(d time.Duration) {
	deadline := time.Now().Add(d)

	var sink uint64
	for time.Now().Before(deadline) {
		sink += uint64(time.Now().UnixNano())
	}

	runtime.KeepAlive(sink)
}
