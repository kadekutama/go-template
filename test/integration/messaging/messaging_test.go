// Package messaging is the E08-T06 G4 slice: NATS fanout via the live
// publisher, the Redpanda produce→consume round-trip against a live
// container, idempotent consumer + DLQ paths (fakes for receipts when the
// Valkey/Postgres inbox is not under test), and webhook endpoint isolation.
// Suites skip without Docker.
package messaging

import (
	"context"
	"errors"
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	edgenats "github.com/kadekutama/go-template/internal/infrastructure/messaging/nats"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/publisher"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	"github.com/kadekutama/go-template/test/doubles"
	mockconsumer "github.com/kadekutama/go-template/test/mock/consumer"
	mockredpanda "github.com/kadekutama/go-template/test/mock/redpanda"
	mockwebhook "github.com/kadekutama/go-template/test/mock/webhook"
	testcontainers "github.com/kadekutama/go-template/test/testcontainers"
)

func testTenant(t *testing.T) valueobject.TenantID {
	t.Helper()

	tenant, err := valueobject.ParseTenantID("01950000-0000-7000-8000-000000000070")
	require.NoError(t, err)

	return tenant
}

func testLedger(t *testing.T) valueobject.LedgerID {
	t.Helper()

	ledger, err := valueobject.ParseLedgerID("01950000-0000-7000-8000-000000000071")
	require.NoError(t, err)

	return ledger
}

func TestPublishConsumeIdempotent(t *testing.T) {
	broker := mockredpanda.NewMockBroker(t)
	var recordedPayload []byte
	broker.EXPECT().Publish(mock.Anything, "outbox.facts.v1", mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context, _ string, _ string, _ map[string]string, payload []byte) {
		recordedPayload = append([]byte(nil), payload...)
	}).Return(nil).Once()

	relay, err := publisher.NewPublisher(publisher.PublisherParams{Broker: broker, Topic: "outbox.facts.v1"})
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, relay.Publish(ctx, appport.OutboxFact{
		TenantID:    testTenant(t),
		LedgerID:    testLedger(t),
		EventType:   "transfer.completed.v1",
		AggregateID: "01950000-0000-7000-8000-000000000072",
		Payload:     []byte(`{"minor":9}`),
		OccurredAt:  time.Now().UTC(),
	}))
	require.NotEmpty(t, recordedPayload)

	receipts := mockconsumer.NewMockReceiptStore(t)
	claims := &doubles.DedupSet{}
	receipts.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c, id string) (bool, error) {
		return claims.Claim(c, id), nil
	}).Maybe()
	receipts.EXPECT().Release(mock.Anything, mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, c, id string) error {
		claims.Release(c, id)
		return nil
	}).Maybe()

	dlq := mockconsumer.NewMockDLQSink(t)

	framework, err := consumer.NewFramework(consumer.FrameworkParams{
		Consumer:    "test-group",
		Receipts:    receipts,
		DLQ:         dlq,
		MaxDelivers: 5,
	})
	require.NoError(t, err)

	runs := 0
	handler := func(_ context.Context, _ appport.Message) error {
		runs++
		return nil
	}

	msg := appport.Message{
		ID:       "fact-1",
		TenantID: testTenant(t),
		Subject:  "ledger.t1.transfer.completed.v1",
		Payload:  recordedPayload,
	}

	require.NoError(t, framework.Handle(ctx, msg, handler))
	require.NoError(t, framework.Handle(ctx, msg, handler))
	assert.Equal(t, 1, runs)
}

func TestConsumerDLQAfterMaxDelivers(t *testing.T) {
	dlq := mockconsumer.NewMockDLQSink(t)
	dlq.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Once()

	receipts := mockconsumer.NewMockReceiptStore(t)
	receipts.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
	receipts.EXPECT().Release(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	framework, err := consumer.NewFramework(consumer.FrameworkParams{
		Consumer:    "test-group",
		Receipts:    receipts,
		DLQ:         dlq,
		MaxDelivers: 3,
	})
	require.NoError(t, err)

	handler := func(_ context.Context, _ appport.Message) error {
		return assert.AnError
	}

	msg := appport.Message{ID: "poison-1", TenantID: testTenant(t), Redelivered: 2}
	require.NoError(t, framework.Handle(context.Background(), msg, handler))
}

// mustTestPolicy builds the api-contracts §11 schedule explicitly for tests.
func mustTestPolicy(t *testing.T) *webhook.RetryPolicy {
	t.Helper()

	policy, err := webhook.NewRetryPolicy([]time.Duration{
		time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour,
		6 * time.Hour, 24 * time.Hour, 48 * time.Hour,
	})
	require.NoError(t, err)

	return policy
}

func TestNATSFanout(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartNATS(t)
	require.NoError(t, err)

	conn, err := natsgo.Connect(handle.URL())
	require.NoError(t, err)
	defer conn.Close()

	received := make(chan []byte, 1)
	_, err = conn.Subscribe("ledger.t1.account.balance.changed.v1", func(msg *natsgo.Msg) {
		cp := make([]byte, len(msg.Data))
		copy(cp, msg.Data)
		received <- cp
	})
	require.NoError(t, err)
	require.NoError(t, conn.Flush())

	publisher, err := edgenats.NewPublisher(edgenats.PublisherParams{
		URL:            handle.URL(),
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 3 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()

	assert.Equal(t, handle.URL(), publisher.URL())
	assert.False(t, publisher.UsesTLS())
	assert.Equal(t, 5*time.Second, publisher.ConnectTimeout())

	ctx := context.Background()
	require.NoError(t, publisher.PublishEvent(ctx, "t1", "account.balance.changed.v1", []byte(`{"minor":3}`)))

	select {
	case got := <-received:
		assert.Equal(t, []byte(`{"minor":3}`), got)
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for NATS fanout (url %s; gate runs the full tree under -race -count=3)", handle.URL())
	}
}

func TestPublisherPublishLive(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartNATS(t)
	require.NoError(t, err)

	edgepublisher, err := edgenats.NewPublisher(edgenats.PublisherParams{
		URL:            handle.URL(),
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 3 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = edgepublisher.Close() }()

	ctx := context.Background()

	raw, err := natsgo.Connect(handle.URL())
	require.NoError(t, err)
	defer raw.Close()

	received := make(chan []byte, 1)
	_, err = raw.Subscribe("ledger.t9.account.balance.changed.v1", func(msg *natsgo.Msg) {
		cp := make([]byte, len(msg.Data))
		copy(cp, msg.Data)
		received <- cp
	})
	require.NoError(t, err)
	require.NoError(t, raw.Flush())

	require.NoError(t, edgepublisher.PublishEvent(ctx, "t9", "account.balance.changed.v1", []byte(`{"minor":8}`)))

	select {
	case got := <-received:
		assert.Equal(t, []byte(`{"minor":8}`), got)
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for publisher publish (url %s; gate runs the full tree under -race -count=3)", handle.URL())
	}
}

func TestPublisherPublishEventValidation(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartNATS(t)
	require.NoError(t, err)

	publisher, err := edgenats.NewPublisher(edgenats.PublisherParams{
		URL:            handle.URL(),
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 3 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()

	ctx := context.Background()

	type testCase struct {
		name          string
		tenant        string
		eventType     string
		payload       []byte
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "nil payload rejected",
			tenant:        "t1",
			eventType:     "account.balance.changed.v1",
			payload:       nil,
			expectedError: errors.New("nats: payload is required"),
		},
		{
			name:          "unversioned event rejected",
			tenant:        "t1",
			eventType:     "transfer",
			payload:       []byte(`{}`),
			expectedError: entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type must be versioned"),
		},
		{
			name:          "blank tenant rejected",
			tenant:        "",
			eventType:     "account.balance.changed.v1",
			payload:       []byte(`{}`),
			expectedError: entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := publisher.PublishEvent(ctx, tc.tenant, tc.eventType, tc.payload)
			assert.EqualError(t, err, tc.expectedError.Error())
		})
	}
}

func TestWebhookIsolation(t *testing.T) {
	ctx := context.Background()
	registry := webhook.NewRegistry(webhook.RegistryParams{})

	tenant := testTenant(t).String()
	require.NoError(t, registry.Upsert(ctx, webhook.Endpoint{
		ID:     "ep-good",
		Tenant: tenant,
		URL:    "https://good.example.com/hook",
		Events: []string{"transfer.completed.v1"},
		Secret: "s-good",
	}))
	require.NoError(t, registry.Upsert(ctx, webhook.Endpoint{
		ID:     "ep-bad",
		Tenant: tenant,
		URL:    "https://bad.example.com/hook",
		Events: []string{"transfer.completed.v1"},
		Secret: "s-bad",
	}))

	sender := mockconsumer.NewMockHTTPSender(t)
	var delivered []string
	sender.EXPECT().Send(mock.Anything, "https://good.example.com/hook", mock.Anything, mock.Anything).Run(func(_ context.Context, u string, _ map[string]string, _ []byte) {
		delivered = append(delivered, u)
	}).Return(nil).Once()
	sender.EXPECT().Send(mock.Anything, "https://bad.example.com/hook", mock.Anything, mock.Anything).Return(assert.AnError).Once()

	dlq := mockwebhook.NewMockDLQSink(t)
	dlq.EXPECT().Record(mock.Anything, mock.Anything).Return(nil).Maybe()

	receipts := mockconsumer.NewMockReceiptStore(t)
	receipts.EXPECT().Claim(mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Maybe()
	receipts.EXPECT().Release(mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
		Endpoints:   registry,
		Sender:      sender,
		Receipts:    receipts,
		DLQ:         dlq,
		RetryPolicy: mustTestPolicy(t),
		Tolerance:   5 * time.Second,
	})
	require.NoError(t, err)

	err = dispatcher.Dispatch(ctx, appport.WebhookMessage{
		TenantID:  testTenant(t),
		EventType: "transfer.completed.v1",
		Payload:   []byte(`{"id":"evt-iso-1"}`),
	})
	assert.Error(t, err)
	assert.Contains(t, delivered, "https://good.example.com/hook")
}
