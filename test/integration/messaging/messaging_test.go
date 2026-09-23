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
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	edgenats "github.com/kadekutama/go-template/internal/infrastructure/messaging/nats"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/consumer"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda/publisher"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	fakes "github.com/kadekutama/go-template/test/fakes"
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
	broker := fakes.NewBroker()
	relay, err := publisher.NewPublisher(publisher.PublisherParams{Broker: broker})
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
	require.Len(t, broker.Records(), 1)

	framework, err := consumer.NewFramework(consumer.FrameworkParams{
		Consumer: "test-group",
		Receipts: fakes.NewReceiptStore(),
		DLQ:      fakes.NewMessageDLQ(),
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
		Payload:  broker.Records()[0].Payload,
	}

	require.NoError(t, framework.Handle(ctx, msg, handler))
	require.NoError(t, framework.Handle(ctx, msg, handler))
	assert.Equal(t, 1, runs)
}

func TestConsumerDLQAfterMaxDelivers(t *testing.T) {
	dlq := fakes.NewMessageDLQ()
	framework, err := consumer.NewFramework(consumer.FrameworkParams{
		Consumer:    "test-group",
		Receipts:    fakes.NewReceiptStore(),
		DLQ:         dlq,
		MaxDelivers: 3,
	})
	require.NoError(t, err)

	handler := func(_ context.Context, _ appport.Message) error {
		return errors.New("poison")
	}

	msg := appport.Message{ID: "poison-1", TenantID: testTenant(t), Redelivered: 2}
	require.NoError(t, framework.Handle(context.Background(), msg, handler))
	assert.Equal(t, 1, dlq.Len())
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

	publisher, err := edgenats.NewPublisher(edgenats.PublisherParams{URL: handle.URL()})
	require.NoError(t, err)
	defer func() { _ = publisher.Close() }()

	assert.Equal(t, handle.URL(), publisher.URL())
	assert.False(t, publisher.UsesTLS())
	assert.Equal(t, edgenats.DefaultConnectTimeout, publisher.ConnectTimeout())

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

	edgepublisher, err := edgenats.NewPublisher(edgenats.PublisherParams{URL: handle.URL()})
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

	publisher, err := edgenats.NewPublisher(edgenats.PublisherParams{URL: handle.URL()})
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

	sender := &flakySender{failURL: "https://bad.example.com/hook"}
	dlq := fakes.NewWebhookDLQ()
	dispatcher, err := consumer.NewDispatcher(consumer.DispatcherParams{
		Endpoints:   registryAdapter{registry: registry},
		Sender:      sender,
		Receipts:    fakes.NewReceiptStore(),
		DLQ:         dlq,
		RetryPolicy: mustTestPolicy(t),
	})
	require.NoError(t, err)

	err = dispatcher.Dispatch(ctx, appport.WebhookMessage{
		TenantID:  testTenant(t),
		EventType: "transfer.completed.v1",
		Payload:   []byte(`{"id":"evt-iso-1"}`),
	})
	assert.Error(t, err)
	assert.Contains(t, sender.delivered, "https://good.example.com/hook")
}

type registryAdapter struct {
	registry *webhook.Registry
}

func (a registryAdapter) Find(ctx context.Context, tenant, event string) []webhook.Endpoint {
	return a.registry.Find(ctx, tenant, event)
}

type flakySender struct {
	failURL   string
	delivered []string
}

func (s *flakySender) Send(_ context.Context, url string, _ map[string]string, _ []byte) error {
	if url == s.failURL {
		return errors.New("endpoint down")
	}

	s.delivered = append(s.delivered, url)

	return nil
}
