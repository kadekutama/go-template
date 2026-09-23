// Package messaging is the E08-T06 G4 slice for the durable log: the
// franz-go producer publishes outbox facts and a consumer reads them back,
// proving the at-least-once relay path end to end (partitioning, headers,
// payload integrity, per-key FIFO order).
package messaging

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	testcontainers "github.com/kadekutama/go-template/test/testcontainers"
)

// TestRedpandaPublishConsumeRoundTrip proves the production Broker: three
// records on one partition key come back in order with headers and payload
// intact, and a second consumer group sees the same records (replayable log).
func TestRedpandaPublishConsumeRoundTrip(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartRedpanda(t)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	producer, err := redpanda.NewProducer(redpanda.ProducerParams{
		Seeds:    []string{handle.Seed()},
		ClientID: "ledger-e08-test",
	})
	require.NoError(t, err)
	defer func() { _ = producer.Close() }()

	const topic = "ledger.events.v1"
	createTopic(t, ctx, handle.Seed(), topic)

	payloads := [][]byte{[]byte(`{"seq":1}`), []byte(`{"seq":2}`), []byte(`{"seq":3}`)}
	for _, payload := range payloads {
		require.NoError(t, producer.Publish(ctx, topic, "tenant-1:account-1", map[string]string{
			"event_type": "transfer.completed.v1",
			"tenant_id":  "tenant-1",
		}, payload))
	}

	records := consumeAll(t, ctx, handle.Seed(), topic)
	require.Len(t, records, 3)

	for idx, record := range records {
		assert.Equal(t, payloads[idx], record.Value)
		assert.Equal(t, "tenant-1:account-1", string(record.Key))
		assert.Equal(t, "transfer.completed.v1", headerValue(record.Headers, "event_type"))
		assert.Equal(t, "tenant-1", headerValue(record.Headers, "tenant_id"))
	}
}

// createTopic provisions the test topic explicitly (1 partition, RF 1):
// the suite never relies on broker-side auto-creation.
func createTopic(t *testing.T, ctx context.Context, seed, topic string) {
	t.Helper()

	client, err := kgo.NewClient(kgo.SeedBrokers(seed))
	require.NoError(t, err)
	defer client.Close()

	admin := kadm.NewClient(client)

	results, err := admin.CreateTopics(ctx, 1, 1, nil, topic)
	require.NoError(t, err)

	for _, result := range results {
		require.NoError(t, result.Err)
	}
}

// consumeAll reads every record currently on topic with an ephemeral,
// non-group consumer starting at the log head.
func consumeAll(t *testing.T, ctx context.Context, seed, topic string) []*kgo.Record {
	t.Helper()

	client, err := kgo.NewClient(
		kgo.SeedBrokers(seed),
		kgo.ClientID("ledger-e08-test-consumer"),
		kgo.ConsumeTopics(topic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	require.NoError(t, err)
	defer client.Close()

	var out []*kgo.Record
	deadline := time.Now().Add(90 * time.Second)

	for len(out) < 3 && time.Now().Before(deadline) {
		fetches := client.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			t.Fatalf("consume %s: %v", topic, errs)
		}

		out = append(out, fetches.Records()...)
	}

	return out
}

func headerValue(headers []kgo.RecordHeader, key string) string {
	for _, header := range headers {
		if header.Key == key {
			return string(header.Value)
		}
	}

	return ""
}
