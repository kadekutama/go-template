package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

type handleObservation struct {
	ran bool
	dlq int
}

func TestNewFramework(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         consumer.FrameworkParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid params",
			params: consumer.FrameworkParams{
				Consumer: "test-group",
				Receipts: fakes.NewReceiptStore(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "blank consumer rejected",
			params: consumer.FrameworkParams{
				Consumer: "",
				Receipts: fakes.NewReceiptStore(),
			},
			expectedResult: false,
			expectedError:  errors.New("consumer: invalid params (1 violation(s)): Consumer: rule \"required\" on value "),
		},
		{
			name: "whitespace consumer rejected",
			params: consumer.FrameworkParams{
				Consumer: "   ",
				Receipts: fakes.NewReceiptStore(),
			},
			expectedResult: false,
			expectedError:  errors.New("consumer: invalid params (1 violation(s)): Consumer: rule \"required\" on value "),
		},
		{
			name: "nil receipts rejected",
			params: consumer.FrameworkParams{
				Consumer: "test-group",
				Receipts: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("consumer: receipt store is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			framework, err := consumer.NewFramework(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, framework)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, framework)
			assert.Equal(t, tc.expectedResult, framework != nil)
		})
	}
}

func TestFrameworkHandleValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		framework     func() *consumer.Framework
		ctx           context.Context
		msg           appport.Message
		handler       appport.MessageHandler
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil framework rejected",
			framework: func() *consumer.Framework {
				return nil
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "m-1",
			},
			handler:       func(_ context.Context, _ appport.Message) error { return nil },
			expectedError: errors.New("consumer: not initialized"),
		},
		{
			name: "blank id rejected",
			framework: func() *consumer.Framework {
				f, _ := consumer.NewFramework(consumer.FrameworkParams{
					Consumer: "g",
					Receipts: fakes.NewReceiptStore(),
				})
				return f
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "",
			},
			handler:       func(_ context.Context, _ appport.Message) error { return nil },
			expectedError: errors.New("consumer: message id is required"),
		},
		{
			name: "whitespace id rejected",
			framework: func() *consumer.Framework {
				f, _ := consumer.NewFramework(consumer.FrameworkParams{
					Consumer: "g",
					Receipts: fakes.NewReceiptStore(),
				})
				return f
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "   ",
			},
			handler:       func(_ context.Context, _ appport.Message) error { return nil },
			expectedError: errors.New("consumer: message id is required"),
		},
		{
			name: "nil handler rejected",
			framework: func() *consumer.Framework {
				f, _ := consumer.NewFramework(consumer.FrameworkParams{
					Consumer: "g",
					Receipts: fakes.NewReceiptStore(),
				})
				return f
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "m-valid",
			},
			handler:       nil,
			expectedError: errors.New("consumer: handler is required"),
		},
		{
			name: "canceled context rejected",
			framework: func() *consumer.Framework {
				f, _ := consumer.NewFramework(consumer.FrameworkParams{
					Consumer: "g",
					Receipts: fakes.NewReceiptStore(),
				})
				return f
			},
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			msg: appport.Message{
				ID: "m-valid",
			},
			handler:       func(_ context.Context, _ appport.Message) error { return nil },
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.framework()
			err := f.Handle(tc.ctx, tc.msg, tc.handler)
			require.Error(t, err)
			if errors.Is(tc.expectedError, context.Canceled) {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}
}

func TestFrameworkHandle(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		msg            appport.Message
		handlerErr     error
		panics         bool
		expectedResult handleObservation
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "handler runs once",
			msg: appport.Message{
				ID: "m-1",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 0,
			},
			handlerErr: nil,
			panics:     false,
			expectedResult: handleObservation{
				ran: true,
				dlq: 0,
			},
			expectedError: nil,
		},
		{
			name: "handler error redelivers",
			msg: appport.Message{
				ID: "m-2",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 0,
			},
			handlerErr: errors.New("boom"),
			panics:     false,
			expectedResult: handleObservation{
				ran: true,
				dlq: 0,
			},
			expectedError: errors.New("boom"),
		},
		{
			name: "exhausted routes to dlq",
			msg: appport.Message{
				ID: "m-3",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 99,
			},
			handlerErr: errors.New("boom"),
			panics:     false,
			expectedResult: handleObservation{
				ran: true,
				dlq: 1,
			},
			expectedError: nil,
		},
		{
			name: "panic recovered as error",
			msg: appport.Message{
				ID: "m-4",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 0,
			},
			handlerErr: nil,
			panics:     true,
			expectedResult: handleObservation{
				ran: true,
				dlq: 0,
			},
			expectedError: errors.New("consumer: handler panic: handler panic"),
		},
		{
			name: "panic exhausted routes to dlq and acks",
			msg: appport.Message{
				ID: "m-5",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 99,
			},
			handlerErr: nil,
			panics:     true,
			expectedResult: handleObservation{
				ran: true,
				dlq: 1,
			},
			expectedError: nil,
		},
		{
			name: "second redelivery still errors",
			msg: appport.Message{
				ID: "m-6",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 1,
			},
			handlerErr: errors.New("boom"),
			panics:     false,
			expectedResult: handleObservation{
				ran: true,
				dlq: 0,
			},
			expectedError: errors.New("boom"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receipts := fakes.NewReceiptStore()
			dlq := fakes.NewMessageDLQ()

			framework, err := consumer.NewFramework(consumer.FrameworkParams{
				Consumer: "test-group",
				Receipts: receipts,
				DLQ:      dlq,
			})
			require.NoError(t, err)

			ran := false
			handler := func(_ context.Context, _ appport.Message) error {
				ran = true
				if tc.panics {
					panic("handler panic")
				}
				return tc.handlerErr
			}

			err = framework.Handle(context.Background(), tc.msg, handler)
			assert.Equal(t, tc.expectedResult, handleObservation{ran: ran, dlq: dlq.Len()})

			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type releaseFailingStore struct {
	inner *fakes.ReceiptStore
	err   error
	calls int
}

func (s *releaseFailingStore) Claim(ctx context.Context, consumer, eventID string) (bool, error) {
	return s.inner.Claim(ctx, consumer, eventID)
}

func (s *releaseFailingStore) Release(_ context.Context, _, _ string) error {
	s.calls++
	return s.err
}

func TestFrameworkRedeliveryReleasesClaim(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		maxDelivers   int
		hasDLQ        bool
		deliveries    []int
		handlerFail   error
		expectedRuns  int
		expectedDLQ   int
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "transient failure then success",
			maxDelivers:   5,
			hasDLQ:        true,
			deliveries:    []int{0, 1, 2},
			handlerFail:   errors.New("handler failed"),
			expectedRuns:  2,
			expectedDLQ:   0,
			expectedError: nil,
		},
		{
			name:          "poison reaches dlq after max delivers",
			maxDelivers:   2,
			hasDLQ:        true,
			deliveries:    []int{0, 1},
			handlerFail:   errors.New("handler failed"),
			expectedRuns:  2,
			expectedDLQ:   1,
			expectedError: nil,
		},
		{
			name:          "exhausted without dlq keeps redelivering",
			maxDelivers:   1,
			hasDLQ:        false,
			deliveries:    []int{5, 6},
			handlerFail:   errors.New("handler failed"),
			expectedRuns:  2,
			expectedDLQ:   0,
			expectedError: errors.New("handler failed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receipts := fakes.NewReceiptStore()
			dlq := fakes.NewMessageDLQ()

			var dlqSink consumer.DLQSink
			if tc.hasDLQ {
				dlqSink = dlq
			}

			framework, err := consumer.NewFramework(consumer.FrameworkParams{
				Consumer:    "test-group",
				Receipts:    receipts,
				DLQ:         dlqSink,
				MaxDelivers: tc.maxDelivers,
			})
			require.NoError(t, err)

			runs := 0
			handler := func(_ context.Context, _ appport.Message) error {
				runs++
				if tc.name == "transient failure then success" && runs > 1 {
					return nil
				}
				return tc.handlerFail
			}

			tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
			var lastErr error
			for _, redelivered := range tc.deliveries {
				msg := appport.Message{
					ID:          "redeliver-test-id",
					TenantID:    tenant,
					Subject:     "ledger.t1.transfer.completed.v1",
					Payload:     []byte(`{"minor":1}`),
					Redelivered: redelivered,
				}
				lastErr = framework.Handle(context.Background(), msg, handler)
			}

			assert.Equal(t, tc.expectedRuns, runs)
			assert.Equal(t, tc.expectedDLQ, dlq.Len())
			if tc.expectedError != nil {
				assert.EqualError(t, lastErr, tc.expectedError.Error())
			} else {
				assert.NoError(t, lastErr)
			}
		})
	}
}

func TestFrameworkReleaseFailureSurfaces(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		redelivered   int
		maxDelivers   int
		hasDLQ        bool
		releaseErr    error
		handlerErr    error
		expectedError error
		expectedCalls int
	}

	testCases := []testCase{
		{
			name:          "stuck receipt surfaces release failure",
			redelivered:   0,
			maxDelivers:   5,
			hasDLQ:        false,
			releaseErr:    errors.New("valkey down"),
			handlerErr:    errors.New("handler failed"),
			expectedError: errors.New("handler failed"),
			expectedCalls: 1,
		},
		{
			name:          "exhausted dlq routing surfaces release failure once",
			redelivered:   5,
			maxDelivers:   5,
			hasDLQ:        true,
			releaseErr:    errors.New("valkey down"),
			handlerErr:    errors.New("handler failed"),
			expectedError: errors.New("handler failed"),
			expectedCalls: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := &releaseFailingStore{inner: fakes.NewReceiptStore(), err: tc.releaseErr}
			var dlq consumer.DLQSink
			if tc.hasDLQ {
				dlq = fakes.NewMessageDLQ()
			}

			framework, err := consumer.NewFramework(consumer.FrameworkParams{
				Consumer:    "test-group",
				Receipts:    store,
				DLQ:         dlq,
				MaxDelivers: tc.maxDelivers,
			})
			require.NoError(t, err)

			handler := func(_ context.Context, _ appport.Message) error { return tc.handlerErr }

			tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
			msg := appport.Message{
				ID:          "stuck-1",
				TenantID:    tenant,
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: tc.redelivered,
			}

			err = framework.Handle(context.Background(), msg, handler)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.handlerErr)
			assert.ErrorIs(t, err, tc.releaseErr)
			assert.Equal(t, tc.expectedCalls, store.calls)
		})
	}
}

func TestFrameworkCorrelation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		payload           []byte
		expectedTraceID   string
		expectedRequestID string
	}

	testCases := []testCase{
		{
			name:              "valid envelope extracts trace and request ids",
			payload:           []byte(`{"tenant_id":"t","event_type":"transfer.completed.v1","trace_id":"trace-9","request_id":"req-9","payload":{}}`),
			expectedTraceID:   "trace-9",
			expectedRequestID: "req-9",
		},
		{
			name:              "non-envelope passes through untouched",
			payload:           []byte(`not-json`),
			expectedTraceID:   "",
			expectedRequestID: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receipts := fakes.NewReceiptStore()
			framework, err := consumer.NewFramework(consumer.FrameworkParams{Consumer: "g", Receipts: receipts})
			require.NoError(t, err)

			var gotTrace, gotRequest string
			handler := func(ctx context.Context, _ appport.Message) error {
				fields := log.FromContext(ctx)
				gotTrace = fields[log.FieldTraceID]
				gotRequest = fields[log.FieldRequestID]
				return nil
			}

			tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
			msg := appport.Message{
				ID:       "corr-msg",
				TenantID: tenant,
				Subject:  "ledger.t1.transfer.completed.v1",
				Payload:  tc.payload,
			}

			require.NoError(t, framework.Handle(context.Background(), msg, handler))
			assert.Equal(t, tc.expectedTraceID, gotTrace)
			assert.Equal(t, tc.expectedRequestID, gotRequest)
		})
	}
}
