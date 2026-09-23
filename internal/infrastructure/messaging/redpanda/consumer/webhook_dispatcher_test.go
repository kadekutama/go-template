package consumer_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

type stubLookup struct {
	endpoints []webhook.Endpoint
}

func (s *stubLookup) Find(_ context.Context, _, _ string) []webhook.Endpoint {
	return s.endpoints
}

type stubSender struct {
	fail    error
	calls   int
	headers []map[string]string
}

func (s *stubSender) Send(_ context.Context, _ string, headers map[string]string, _ []byte) error {
	s.calls++
	s.headers = append(s.headers, headers)
	return s.fail
}

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
				Endpoints:   &stubLookup{},
				Sender:      &stubSender{},
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         fakes.NewWebhookDLQ(),
				RetryPolicy: policy,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil endpoints rejected",
			params: consumer.DispatcherParams{
				Endpoints:   nil,
				Sender:      &stubSender{},
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         fakes.NewWebhookDLQ(),
				RetryPolicy: policy,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: endpoint lookup is required"),
		},
		{
			name: "nil sender rejected",
			params: consumer.DispatcherParams{
				Endpoints:   &stubLookup{},
				Sender:      nil,
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         fakes.NewWebhookDLQ(),
				RetryPolicy: policy,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: sender is required"),
		},
		{
			name: "nil receipts rejected",
			params: consumer.DispatcherParams{
				Endpoints:   &stubLookup{},
				Sender:      &stubSender{},
				Receipts:    nil,
				DLQ:         fakes.NewWebhookDLQ(),
				RetryPolicy: policy,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: receipt store is required"),
		},
		{
			name: "nil dlq rejected",
			params: consumer.DispatcherParams{
				Endpoints:   &stubLookup{},
				Sender:      &stubSender{},
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         nil,
				RetryPolicy: policy,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: dlq sink is required"),
		},
		{
			name: "nil retry policy rejected",
			params: consumer.DispatcherParams{
				Endpoints:   &stubLookup{},
				Sender:      &stubSender{},
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         fakes.NewWebhookDLQ(),
				RetryPolicy: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("dispatcher: retry policy is required"),
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
			name: "blank tenant rejected",
			dispatcher: func() *consumer.Dispatcher {
				pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
				d, _ := consumer.NewDispatcher(consumer.DispatcherParams{
					Endpoints:   &stubLookup{},
					Sender:      &stubSender{},
					Receipts:    fakes.NewReceiptStore(),
					DLQ:         fakes.NewWebhookDLQ(),
					RetryPolicy: pol,
				})
				return d
			},
			ctx: context.Background(),
			message: appport.WebhookMessage{
				TenantID:  valueobject.TenantID(""),
				EventType: "transfer.completed.v1",
				Payload:   []byte(`{"id":"evt-val"}`),
			},
			expectedError: errors.New("dispatcher: tenant is required"),
		},
		{
			name: "blank event rejected",
			dispatcher: func() *consumer.Dispatcher {
				pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
				d, _ := consumer.NewDispatcher(consumer.DispatcherParams{
					Endpoints:   &stubLookup{},
					Sender:      &stubSender{},
					Receipts:    fakes.NewReceiptStore(),
					DLQ:         fakes.NewWebhookDLQ(),
					RetryPolicy: pol,
				})
				return d
			},
			ctx: context.Background(),
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
			name: "whitespace event rejected",
			dispatcher: func() *consumer.Dispatcher {
				pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
				d, _ := consumer.NewDispatcher(consumer.DispatcherParams{
					Endpoints:   &stubLookup{},
					Sender:      &stubSender{},
					Receipts:    fakes.NewReceiptStore(),
					DLQ:         fakes.NewWebhookDLQ(),
					RetryPolicy: pol,
				})
				return d
			},
			ctx: context.Background(),
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
			name: "nil payload rejected",
			dispatcher: func() *consumer.Dispatcher {
				pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
				d, _ := consumer.NewDispatcher(consumer.DispatcherParams{
					Endpoints:   &stubLookup{},
					Sender:      &stubSender{},
					Receipts:    fakes.NewReceiptStore(),
					DLQ:         fakes.NewWebhookDLQ(),
					RetryPolicy: pol,
				})
				return d
			},
			ctx: context.Background(),
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
			name: "canceled context rejected",
			dispatcher: func() *consumer.Dispatcher {
				pol, _ := webhook.NewRetryPolicy([]time.Duration{time.Minute})
				d, _ := consumer.NewDispatcher(consumer.DispatcherParams{
					Endpoints:   &stubLookup{},
					Sender:      &stubSender{},
					Receipts:    fakes.NewReceiptStore(),
					DLQ:         fakes.NewWebhookDLQ(),
					RetryPolicy: pol,
				})
				return d
			},
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

			sender := &stubSender{fail: tc.senderFail}
			dlq := fakes.NewWebhookDLQ()
			dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
				Endpoints:   &stubLookup{endpoints: tc.endpoints},
				Sender:      sender,
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         dlq,
				RetryPolicy: policy,
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

			assert.Equal(t, tc.expectedCalls, sender.calls)
			assert.Equal(t, tc.expectedDLQ, dlq.Len())

			if tc.verifyHeaders != nil {
				tc.verifyHeaders(t, sender.headers)
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

			lookup := &stubLookup{endpoints: []webhook.Endpoint{endpoint}}
			sender := &stubSender{fail: errors.New("down")}
			dlq := fakes.NewWebhookDLQ()

			dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
				Endpoints:   lookup,
				Sender:      sender,
				Receipts:    fakes.NewReceiptStore(),
				DLQ:         dlq,
				RetryPolicy: policy,
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
			require.Equal(t, 1, dlq.Len())
			assert.Equal(t, tc.expectedAttempts, dlq.List()[0].Attempts)
		})
	}
}
