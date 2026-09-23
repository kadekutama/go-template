package publisher_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/publisher"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	fakes "github.com/kadekutama/go-template/test/fakes"
)

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         publisher.PublisherParams
		expectedResult bool
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "broker accepted",
			params: publisher.PublisherParams{
				Broker: fakes.NewBroker(),
			},
			expectedResult: true,
			expectedError:  nil,
		},
		{
			name: "nil broker rejected",
			params: publisher.PublisherParams{
				Broker: nil,
			},
			expectedResult: false,
			expectedError:  errors.New("publisher: broker is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instance, err := publisher.NewPublisher(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, instance)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, instance)
			assert.Equal(t, tc.expectedResult, instance != nil)
		})
	}
}

func TestPublisherPublish(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		publisher         func(broker *fakes.Broker) *publisher.Publisher
		armBrokerFail     error
		ctx               context.Context
		facts             []appport.OutboxFact
		expectedRecords   int
		expectedEventType string
		expectedError     error
	}

	testCases := []testCase{
		{
			name: "nil publisher rejected",
			publisher: func(_ *fakes.Broker) *publisher.Publisher {
				return nil
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: not initialized"),
		},
		{
			name: "relays valid fact",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   1,
			expectedEventType: "transfer.completed.v1",
			expectedError:     nil,
		},
		{
			name: "missing tenant rejected",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: valueobject.TenantID(""),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: tenant is required"),
		},
		{
			name: "missing event type rejected",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: event type is required"),
		},
		{
			name: "missing aggregate rejected",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: aggregate id is required"),
		},
		{
			name: "nil payload rejected",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     nil,
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: payload is required"),
		},
		{
			name: "broker failure surfaces",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: errors.New("broker down"),
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
				},
			},
			expectedRecords:   0,
			expectedEventType: "",
			expectedError:     errors.New("publisher: publish transfer.completed.v1: broker down"),
		},
		{
			name: "zero occurred at defaults to current time",
			publisher: func(b *fakes.Broker) *publisher.Publisher {
				p, _ := publisher.NewPublisher(publisher.PublisherParams{Broker: b})
				return p
			},
			armBrokerFail: nil,
			ctx:           context.Background(),
			facts: []appport.OutboxFact{
				{
					TenantID: func() valueobject.TenantID {
						t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
						return t
					}(),
					LedgerID: func() valueobject.LedgerID {
						l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
						return l
					}(),
					EventType:   "transfer.completed.v1",
					AggregateID: "01950000-0000-7000-8000-000000000030",
					Payload:     []byte(`{"minor":100}`),
					OccurredAt:  time.Time{},
				},
			},
			expectedRecords:   1,
			expectedEventType: "transfer.completed.v1",
			expectedError:     nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			if tc.armBrokerFail != nil {
				broker.SetFail(tc.armBrokerFail)
			}
			p := tc.publisher(broker)
			err := p.Publish(tc.ctx, tc.facts...)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			records := broker.Records()
			assert.Len(t, records, tc.expectedRecords)
			if tc.expectedRecords > 0 {
				assert.Equal(t, tc.expectedEventType, records[0].Headers["event_type"])
			}
		})
	}
}

func TestPublisherPropagatesCorrelation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		traceID           string
		requestID         string
		expectedTraceID   string
		expectedRequestID string
		expectedInBody    bool
	}

	testCases := []testCase{
		{
			name:              "trace and request ids propagate",
			traceID:           "trace-1",
			requestID:         "req-1",
			expectedTraceID:   "trace-1",
			expectedRequestID: "req-1",
			expectedInBody:    true,
		},
		{
			name:              "empty correlation still relays",
			traceID:           "",
			requestID:         "",
			expectedTraceID:   "",
			expectedRequestID: "",
			expectedInBody:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			instance, err := publisher.NewPublisher(publisher.PublisherParams{Broker: broker})
			require.NoError(t, err)

			ctx := log.WithTraceID(log.WithRequestID(context.Background(), tc.requestID), tc.traceID)
			fact := appport.OutboxFact{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
					return t
				}(),
				LedgerID: func() valueobject.LedgerID {
					l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
					return l
				}(),
				EventType:   "transfer.completed.v1",
				AggregateID: "01950000-0000-7000-8000-000000000031",
				Payload:     []byte(`{"minor":2}`),
				OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
			}

			err = instance.Publish(ctx, fact)
			require.NoError(t, err)

			records := broker.Records()
			require.Len(t, records, 1)
			assert.Equal(t, tc.expectedTraceID, records[0].Headers["trace_id"])
			assert.Equal(t, tc.expectedRequestID, records[0].Headers["request_id"])

			body := string(records[0].Payload)
			assert.Equal(t, tc.expectedInBody, strings.Contains(body, tc.traceID) && strings.Contains(body, tc.requestID))
		})
	}
}

func TestPublisherConfiguredTopic(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		topic         string
		expectedTopic string
	}

	testCases := []testCase{
		{
			name:          "unset topic uses default",
			topic:         "",
			expectedTopic: redpanda.DefaultTopicsConfig().OutboxFacts,
		},
		{
			name:          "configured topic wins",
			topic:         "tenant.outbox.facts.v2",
			expectedTopic: "tenant.outbox.facts.v2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			instance, err := publisher.NewPublisher(publisher.PublisherParams{
				Broker: broker,
				Topic:  tc.topic,
			})
			require.NoError(t, err)

			fact := appport.OutboxFact{
				TenantID: func() valueobject.TenantID {
					t, _ := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000010")
					return t
				}(),
				LedgerID: func() valueobject.LedgerID {
					l, _ := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000020")
					return l
				}(),
				EventType:   "transfer.completed.v1",
				AggregateID: "01950000-0000-7000-8000-000000000031",
				Payload:     []byte(`{"minor":1}`),
				OccurredAt:  time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC),
			}

			require.NoError(t, instance.Publish(context.Background(), fact))
			require.Len(t, broker.Records(), 1)
			assert.Equal(t, tc.expectedTopic, broker.Records()[0].Topic)
		})
	}
}

func TestPublisherEmptyBatch(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		facts           []appport.OutboxFact
		expectedRecords int
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "empty batch succeeds without io",
			facts:           []appport.OutboxFact{},
			expectedRecords: 0,
			expectedError:   nil,
		},
		{
			name:            "nil batch succeeds without io",
			facts:           nil,
			expectedRecords: 0,
			expectedError:   nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			broker := fakes.NewBroker()
			instance, err := publisher.NewPublisher(publisher.PublisherParams{Broker: broker})
			require.NoError(t, err)

			err = instance.Publish(context.Background(), tc.facts...)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			assert.Len(t, broker.Records(), tc.expectedRecords)
		})
	}
}
