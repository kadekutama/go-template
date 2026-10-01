package consumer_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	"github.com/kadekutama/go-template/test/doubles"
	mockconsumer "github.com/kadekutama/go-template/test/mock/consumer"
	mockwebhook "github.com/kadekutama/go-template/test/mock/webhook"
)

func TestNewDispatcher(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         consumer.DispatcherParams
		expectedResult bool
		expectedError  error
	}

	policy, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})

	testCases := []testCase{
		{
			name: "valid params",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil endpoints rejected",
			params: consumer.DispatcherParams{
				Endpoints:   nil,
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: endpoint lookup is required"),
		},
		{
			name: "nil sender rejected",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      nil,
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: sender is required"),
		},
		{
			name: "nil receipts rejected",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    nil,
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: receipt store is required"),
		},
		{
			name: "nil dlq rejected",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         nil,
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: dlq sink is required"),
		},
		{
			name: "nil retry policy rejected",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: nil,
				Tolerance:   5 * time.Minute,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: retry policy is required"),
		},
		{
			name: "non-positive tolerance rejected",
			params: consumer.DispatcherParams{
				Endpoints:   mockconsumer.NewMockEndpointLookup(t),
				Sender:      mockconsumer.NewMockHTTPSender(t),
				Receipts:    mockconsumer.NewMockReceiptStore(t),
				DLQ:         mockwebhook.NewMockDLQSink(t),
				RetryPolicy: policy,
				Tolerance:   0,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: tolerance must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dispatcher, err := consumer.NewDispatcher(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, dispatcher)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, dispatcher)
			assert.Equal(t, tc.expectedResult, dispatcher != nil)
		})
	}
}

func TestDispatcherDispatchValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		dispatcher    func() *consumer.Dispatcher
		ctx           context.Context
		message       appport.WebhookMessage
		expectedError error
	}

	newValidDispatcher := func() *consumer.Dispatcher {
		pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
		d, err := consumer.NewDispatcher(consumer.DispatcherParams{
			Endpoints:   mockconsumer.NewMockEndpointLookup(t),
			Sender:      mockconsumer.NewMockHTTPSender(t),
			Receipts:    mockconsumer.NewMockReceiptStore(t),
			DLQ:         mockwebhook.NewMockDLQSink(t),
			RetryPolicy: pol,
			Tolerance:   5 * time.Minute,
		})
		require.NoError(t, err)
		return d
	}

	testCases := []testCase{
		{
			name: "nil dispatcher rejected",
			dispatcher: func() *consumer.Dispatcher {
				return nil
			},
			ctx: context.Background(),
			message: appport.WebhookMessage{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
					return t
				}(),
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: errors.New("dispatcher: not initialized"),
		},
		{
			name:       "blank tenant rejected",
			dispatcher: newValidDispatcher,
			ctx:        context.Background(),
			message: appport.WebhookMessage{
				TenantID:  valueobject.TenantID(""),
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: errors.New("dispatcher: tenant is required"),
		},
		{
			name:       "blank event rejected",
			dispatcher: newValidDispatcher,
			ctx:        context.Background(),
			message: appport.WebhookMessage{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
					return t
				}(),
				EventType: "",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: errors.New("dispatcher: event type is required"),
		},
		{
			name:       "whitespace event rejected",
			dispatcher: newValidDispatcher,
			ctx:        context.Background(),
			message: appport.WebhookMessage{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
					return t
				}(),
				EventType: "   ",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: errors.New("dispatcher: event type is required"),
		},
		{
			name:       "nil payload rejected",
			dispatcher: newValidDispatcher,
			ctx:        context.Background(),
			message: appport.WebhookMessage{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
					return t
				}(),
				EventType: "transfer.completed.v1",
				Payload:   nil,
			},
			expectedError: errors.New("dispatcher: payload is required"),
		},
		{
			name:       "canceled context rejected",
			dispatcher: newValidDispatcher,
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			message: appport.WebhookMessage{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
					return t
				}(),
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			d := tc.dispatcher()
			err := d.Dispatch(tc.ctx, tc.message)
			require.Error(t, err)
			if errors.Is(tc.expectedError, context.Canceled) {
				assert.ErrorIs(t, err, tc.expectedError)
			} else {
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}
}

func TestDispatcherDispatchScenarios(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		endpoints     []webhook.Endpoint
		senderFail    error
		repeatSend    int
		expectedCalls int
		expectedDLQ   int
		expectedError error
		verifyHeaders func(t *testing.T, headers []map[string]string)
	}

	testCases := []testCase{
		{
			name: "delivers once and dedupes",
			endpoints: []webhook.Endpoint{
				{
					ID:     "ep-1",
					Tenant: "01950000-0000-7000-8000-000000000050",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
					Active: true,
				},
			},
			senderFail:    nil,
			repeatSend:    2,
			expectedCalls: 1,
			expectedDLQ:   0,
			expectedError: nil,
			verifyHeaders: nil,
		},
		{
			name: "contract headers match api-contracts section 11",
			endpoints: []webhook.Endpoint{
				{
					ID:     "ep-1",
					Tenant: "01950000-0000-7000-8000-000000000050",
					URL:    "https://m.example.com/hook",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
					Active: true,
				},
			},
			senderFail:    nil,
			repeatSend:    1,
			expectedCalls: 1,
			expectedDLQ:   0,
			expectedError: nil,
			verifyHeaders: func(t *testing.T, headers []map[string]string) {
				require.Len(t, headers, 1)
				h := headers[0]
				assert.Contains(t, h["X-Ledger-Signature"], "v1=")
				assert.NotEmpty(t, h["X-Ledger-Timestamp"])
				assert.NotEmpty(t, h["X-Ledger-Event-ID"])
				assert.Equal(t, "k1", h["X-Ledger-Key-ID"])
				assert.NotContains(t, h, "X-Signature")
			},
		},
		{
			name: "failing endpoint does not block healthy",
			endpoints: []webhook.Endpoint{
				{
					ID:     "ep-bad",
					Tenant: "01950000-0000-7000-8000-000000000050",
					URL:    "https://m.example.com/hook/bad",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
					Active: true,
				},
				{
					ID:     "ep-good",
					Tenant: "01950000-0000-7000-8000-000000000050",
					URL:    "https://m.example.com/hook/good",
					Events: []string{"transfer.completed.v1"},
					Secret: "s1",
					Active: true,
				},
			},
			senderFail:    errors.New("down"),
			repeatSend:    1,
			expectedCalls: 2,
			expectedDLQ:   0,
			expectedError: errors.New("down"),
			verifyHeaders: nil,
		},
		{
			name:          "no endpoints succeeds",
			endpoints:     []webhook.Endpoint{},
			senderFail:    nil,
			repeatSend:    1,
			expectedCalls: 0,
			expectedDLQ:   0,
			expectedError: nil,
			verifyHeaders: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := webhook.NewRetryPolicy([]time.Duration{
				time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour,
				6 * time.Hour, 24 * time.Hour, 48 * time.Hour,
			})
			require.NoError(t, err)

			lookup := mockconsumer.NewMockEndpointLookup(t)
			lookup.EXPECT().Find(mock.Anything, mock.Anything, mock.Anything).Return(tc.endpoints).Maybe()

			sender := mockconsumer.NewMockHTTPSender(t)
			var calls atomic.Int32
			headerCh := make(chan map[string]string, 100)
			sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, _ string, headers map[string]string, _ []byte) error {
					calls.Add(1)
					headerCh <- headers
					return tc.senderFail
				}).Maybe()

			receipts := mockconsumer.NewMockReceiptStore(t)
			seen := &doubles.DedupSet{}
			receipts.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, consumerID, eventID string) (bool, error) {
					return seen.Claim(consumerID, eventID), nil
				}).Maybe()
			receipts.EXPECT().Release(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, consumerID, eventID string) error {
					seen.Release(consumerID, eventID)
					return nil
				}).Maybe()

			dlq := mockwebhook.NewMockDLQSink(t)
			recordedDLQ := &doubles.MessageLog[webhook.DLQMessage]{}
			dlq.EXPECT().Record(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, msg webhook.DLQMessage) error {
					recordedDLQ.Add(msg)
					return nil
				}).Maybe()

			dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
				Endpoints:   lookup,
				Sender:      sender,
				Receipts:    receipts,
				DLQ:         dlq,
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			})
			require.NoError(t, err)

			tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
			msg := appport.WebhookMessage{
				TenantID:  tenant,
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-1"}`),
			}

			var lastErr error
			for i := 0; i < tc.repeatSend; i++ {
				lastErr = dispatcher.Dispatch(context.Background(), msg)
			}

			if tc.expectedError != nil {
				require.Error(t, lastErr)
			} else {
				assert.NoError(t, lastErr)
			}

			dlqLen := len(recordedDLQ.All())

			assert.Equal(t, tc.expectedCalls, int(calls.Load()))
			assert.Equal(t, tc.expectedDLQ, dlqLen)

			close(headerCh)
			var recordedHeaders []map[string]string
			for h := range headerCh {
				recordedHeaders = append(recordedHeaders, h)
			}
			if tc.verifyHeaders != nil {
				tc.verifyHeaders(t, recordedHeaders)
			}
		})
	}
}

func TestDispatcherRetryScheduleAndDLQ(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		retrySteps       []time.Duration
		expectedAttempts int
	}

	testCases := []testCase{
		{
			name: "custom two-step policy reaches dlq on 3rd attempt",
			retrySteps: []time.Duration{
				time.Second,
				2 * time.Second,
			},
			expectedAttempts: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := webhook.Endpoint{
				ID:     "ep-policy",
				Tenant: "01950000-0000-7000-8000-000000000050",
				URL:    "https://m.example.com/hook/policy",
				Events: []string{"transfer.completed.v1"},
				Secret: "s1",
				Active: true,
			}

			policy, err := webhook.NewRetryPolicy(tc.retrySteps)
			require.NoError(t, err)

			lookup := mockconsumer.NewMockEndpointLookup(t)
			lookup.EXPECT().Find(mock.Anything, mock.Anything, mock.Anything).Return([]webhook.Endpoint{endpoint}).Maybe()

			sender := mockconsumer.NewMockHTTPSender(t)
			sender.EXPECT().Send(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("down")).Maybe()

			receipts := mockconsumer.NewMockReceiptStore(t)
			seen := &doubles.DedupSet{}
			receipts.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, consumerID, eventID string) (bool, error) {
					return seen.Claim(consumerID, eventID), nil
				}).Maybe()
			receipts.EXPECT().Release(mock.Anything, mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, consumerID, eventID string) error {
					seen.Release(consumerID, eventID)
					return nil
				}).Maybe()

			dlq := mockwebhook.NewMockDLQSink(t)
			recordedDLQ := &doubles.MessageLog[webhook.DLQMessage]{}
			dlq.EXPECT().Record(mock.Anything, mock.Anything).
				RunAndReturn(func(_ context.Context, msg webhook.DLQMessage) error {
					recordedDLQ.Add(msg)
					return nil
				}).Maybe()

			dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
				Endpoints:   lookup,
				Sender:      sender,
				Receipts:    receipts,
				DLQ:         dlq,
				RetryPolicy: policy,
				Tolerance:   5 * time.Minute,
			})
			require.NoError(t, err)

			tenant, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000050")
			msg := appport.WebhookMessage{
				TenantID:  tenant,
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-custom-policy"}`),
			}

			for attempt := 1; attempt <= len(tc.retrySteps); attempt++ {
				err := dispatcher.Dispatch(context.Background(), msg)
				require.Error(t, err)

				var retryable *webhook.RetryableError
				require.ErrorAs(t, err, &retryable)
				assert.Equal(t, attempt, retryable.Attempt)
			}

			require.NoError(t, dispatcher.Dispatch(context.Background(), msg))
			dlqMsgs := recordedDLQ.All()
			require.Equal(t, 1, len(dlqMsgs))
			assert.Equal(t, tc.expectedAttempts, dlqMsgs[0].Attempts)
		})
	}
}
