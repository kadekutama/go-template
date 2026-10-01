package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	mockredpanda "github.com/kadekutama/go-template/test/mock/redpanda"
)

func newTestTopicRegistry(t *testing.T) *redpanda.TopicRegistry {
	t.Helper()

	reg, err := redpanda.NewTopicRegistry(redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	})
	require.NoError(t, err)

	return reg
}

func TestNewRedpandaMessageDLQ(t *testing.T) {
	t.Parallel()

	topics := newTestTopicRegistry(t)

	type testCase struct {
		name           string
		params         consumer.RedpandaMessageDLQParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid params",
			params: consumer.RedpandaMessageDLQParams{
				Broker: mockredpanda.NewMockBroker(t),
				Topics: topics,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil broker rejected",
			params: consumer.RedpandaMessageDLQParams{
				Broker: nil,
				Topics: topics,
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: broker is required"),
		},
		{
			name: "nil topics rejected",
			params: consumer.RedpandaMessageDLQParams{
				Broker: mockredpanda.NewMockBroker(t),
				Topics: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: topic registry is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink, err := consumer.NewRedpandaMessageDLQ(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, sink)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sink)
			assert.Equal(t, tc.expectedResult, sink != nil)
		})
	}
}

func TestRedpandaMessageDLQRecord(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		sink           func(b redpanda.Broker) *consumer.RedpandaMessageDLQ
		ctx            context.Context
		msg            appport.Message
		expectedTopic  string
		expectedKey    string
		expectedSource string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil sink rejected",
			sink: func(_ redpanda.Broker) *consumer.RedpandaMessageDLQ {
				return nil
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "dlq-msg-1",
			},
			expectedTopic:  "",
			expectedKey:    "",
			expectedSource: "",
			expectedError:  errors.New("dlq: not initialized"),
		},
		{
			name: "records message to dlq",
			sink: func(b redpanda.Broker) *consumer.RedpandaMessageDLQ {
				s, _ := consumer.NewRedpandaMessageDLQ(consumer.RedpandaMessageDLQParams{
					Broker: b,
					Topics: newTestTopicRegistry(t),
				})
				return s
			},
			ctx: context.Background(),
			msg: appport.Message{
				ID: "dlq-msg-1",
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000040")
					return t
				}(),
				Subject:     "ledger.t1.transfer.completed.v1",
				Payload:     []byte(`{"minor":1}`),
				Redelivered: 7,
			},
			expectedTopic:  "ledger.events.v1.dlq",
			expectedKey:    "01950000-0000-7000-8000-000000000040:dlq-msg-1",
			expectedSource: "consumer",
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := mockredpanda.NewMockBroker(t)
			var recordedTopic string
			var recordedKey string
			var recordedHeaders map[string]string
			var recordedPayload []byte

			broker.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, topic, key string, headers map[string]string, payload []byte) error {
				recordedTopic = topic
				recordedKey = key
				recordedHeaders = make(map[string]string, len(headers))
				for k, v := range headers {
					recordedHeaders[k] = v
				}
				recordedPayload = append([]byte(nil), payload...)
				return nil
			}).Maybe()

			sink := tc.sink(broker)
			err := sink.Record(tc.ctx, tc.msg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTopic, recordedTopic)
			assert.Equal(t, tc.expectedKey, recordedKey)
			assert.Equal(t, tc.expectedSource, recordedHeaders["dlq_source"])
			assert.Contains(t, string(recordedPayload), `"redelivered":7`)
			assert.Contains(t, string(recordedPayload), tc.msg.ID)
		})
	}
}

func TestNewRedpandaWebhookDLQ(t *testing.T) {
	t.Parallel()

	topics := newTestTopicRegistry(t)

	type testCase struct {
		name           string
		params         consumer.RedpandaWebhookDLQParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid params",
			params: consumer.RedpandaWebhookDLQParams{
				Broker: mockredpanda.NewMockBroker(t),
				Topics: topics,
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil broker rejected",
			params: consumer.RedpandaWebhookDLQParams{
				Broker: nil,
				Topics: topics,
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: broker is required"),
		},
		{
			name: "nil topics rejected",
			params: consumer.RedpandaWebhookDLQParams{
				Broker: mockredpanda.NewMockBroker(t),
				Topics: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: topic registry is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink, err := consumer.NewRedpandaWebhookDLQ(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, sink)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sink)
			assert.Equal(t, tc.expectedResult, sink != nil)
		})
	}
}

func TestRedpandaWebhookDLQRecord(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		sink           func(b redpanda.Broker) *consumer.RedpandaWebhookDLQ
		ctx            context.Context
		msg            webhook.DLQMessage
		expectedTopic  string
		expectedSource string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil sink rejected",
			sink: func(_ redpanda.Broker) *consumer.RedpandaWebhookDLQ {
				return nil
			},
			ctx: context.Background(),
			msg: webhook.DLQMessage{
				EndpointID: "ep-1",
			},
			expectedTopic:  "",
			expectedSource: "",
			expectedError:  errors.New("dlq: not initialized"),
		},
		{
			name: "records webhook delivery to dlq",
			sink: func(b redpanda.Broker) *consumer.RedpandaWebhookDLQ {
				s, _ := consumer.NewRedpandaWebhookDLQ(consumer.RedpandaWebhookDLQParams{
					Broker: b,
					Topics: newTestTopicRegistry(t),
				})
				return s
			},
			ctx: context.Background(),
			msg: webhook.DLQMessage{
				EndpointID: "ep-1",
				Tenant:     "01950000-0000-7000-8000-000000000080",
				Event:      "transfer.completed.v1",
				Payload:    []byte(`{"id":"e"}`),
				Attempts:   8,
			},
			expectedTopic:  "webhook.jobs.v1.dlq",
			expectedSource: "webhook-dispatcher",
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := mockredpanda.NewMockBroker(t)
			var recordedTopic string
			var recordedHeaders map[string]string
			var recordedPayload []byte

			broker.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, topic, key string, headers map[string]string, payload []byte) error {
				recordedTopic = topic
				recordedHeaders = make(map[string]string, len(headers))
				for k, v := range headers {
					recordedHeaders[k] = v
				}
				recordedPayload = append([]byte(nil), payload...)
				return nil
			}).Maybe()

			sink := tc.sink(broker)
			err := sink.Record(tc.ctx, tc.msg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedTopic, recordedTopic)
			assert.Contains(t, string(recordedPayload), `"attempts":8`)
			assert.Equal(t, tc.expectedSource, recordedHeaders["dlq_source"])
		})
	}
}
