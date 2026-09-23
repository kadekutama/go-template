package consumer_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

func TestNewRedpandaMessageDLQ(t *testing.T) {
	t.Parallel()

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
				Broker: fakes.NewBroker(),
				Topics: redpanda.DefaultTopicRegistry(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil broker rejected",
			params: consumer.RedpandaMessageDLQParams{
				Broker: nil,
				Topics: redpanda.DefaultTopicRegistry(),
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: broker is required"),
		},
		{
			name: "nil topics rejected",
			params: consumer.RedpandaMessageDLQParams{
				Broker: fakes.NewBroker(),
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
		sink           func(b *fakes.Broker) *consumer.RedpandaMessageDLQ
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
			sink: func(_ *fakes.Broker) *consumer.RedpandaMessageDLQ {
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
			sink: func(b *fakes.Broker) *consumer.RedpandaMessageDLQ {
				s, _ := consumer.NewRedpandaMessageDLQ(consumer.RedpandaMessageDLQParams{
					Broker: b,
					Topics: redpanda.DefaultTopicRegistry(),
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
			expectedTopic:  redpanda.DefaultTopicsConfig().LedgerEvents + ".dlq",
			expectedKey:    "01950000-0000-7000-8000-000000000040:dlq-msg-1",
			expectedSource: "consumer",
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			sink := tc.sink(broker)
			err := sink.Record(tc.ctx, tc.msg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			records := broker.Records()
			require.Len(t, records, 1)
			assert.Equal(t, tc.expectedTopic, records[0].Topic)
			assert.Equal(t, tc.expectedKey, records[0].Key)
			assert.Equal(t, tc.expectedSource, records[0].Headers["dlq_source"])
			assert.Contains(t, string(records[0].Payload), `"redelivered":7`)
			assert.Contains(t, string(records[0].Payload), tc.msg.ID)
		})
	}
}

func TestNewRedpandaWebhookDLQ(t *testing.T) {
	t.Parallel()

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
				Broker: fakes.NewBroker(),
				Topics: redpanda.DefaultTopicRegistry(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil broker rejected",
			params: consumer.RedpandaWebhookDLQParams{
				Broker: nil,
				Topics: redpanda.DefaultTopicRegistry(),
			},
			expectedResult: false,
			expectedError:  errors.New("dlq: broker is required"),
		},
		{
			name: "nil topics rejected",
			params: consumer.RedpandaWebhookDLQParams{
				Broker: fakes.NewBroker(),
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
		sink           func(b *fakes.Broker) *consumer.RedpandaWebhookDLQ
		ctx            context.Context
		msg            webhook.DLQMessage
		expectedTopic  string
		expectedSource string
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil sink rejected",
			sink: func(_ *fakes.Broker) *consumer.RedpandaWebhookDLQ {
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
			sink: func(b *fakes.Broker) *consumer.RedpandaWebhookDLQ {
				s, _ := consumer.NewRedpandaWebhookDLQ(consumer.RedpandaWebhookDLQParams{
					Broker: b,
					Topics: redpanda.DefaultTopicRegistry(),
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
			expectedTopic:  redpanda.DefaultTopicsConfig().WebhookJobs + ".dlq",
			expectedSource: "webhook-dispatcher",
			expectedError:  nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			sink := tc.sink(broker)
			err := sink.Record(tc.ctx, tc.msg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			records := broker.Records()
			require.Len(t, records, 1)
			assert.Equal(t, tc.expectedTopic, records[0].Topic)
			assert.Contains(t, string(records[0].Payload), `"attempts":8`)
			assert.Equal(t, tc.expectedSource, records[0].Headers["dlq_source"])
		})
	}
}
