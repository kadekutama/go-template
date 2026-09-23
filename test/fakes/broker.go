// Package fakes holds in-memory test doubles for the messaging adapters.
// It lives under test/ on purpose: production packages never import it, so
// no binary can accidentally select an in-memory store for durable data.
// Production paths use the Redpanda/Valkey/PostgreSQL adapters; this package
// exists so unit and integration tests can exercise the real interfaces
// without a broker.
package fakes

import (
	"context"
	"sync"
)

// PublishedRecord is one broker write observed by Broker.
type PublishedRecord struct {
	Topic   string
	Key     string
	Headers map[string]string
	Payload []byte
}

// Broker is an in-memory redpanda.Broker test double. It is safe for
// concurrent use.
type Broker struct {
	mu      sync.Mutex
	records []PublishedRecord
	fail    error
}

// NewBroker builds the double.
func NewBroker() *Broker {
	return &Broker{}
}

// SetFail makes subsequent publishes fail with err (nil clears).
func (b *Broker) SetFail(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.fail = err
}

// Publish appends the record or fails when armed.
func (b *Broker) Publish(_ context.Context, topic, key string, headers map[string]string, payload []byte) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.fail != nil {
		return b.fail
	}

	cpHeaders := make(map[string]string, len(headers))
	for k, v := range headers {
		cpHeaders[k] = v
	}

	cpPayload := make([]byte, len(payload))
	copy(cpPayload, payload)

	b.records = append(b.records, PublishedRecord{Topic: topic, Key: key, Headers: cpHeaders, Payload: cpPayload})

	return nil
}

// Records returns a copy of published records.
func (b *Broker) Records() []PublishedRecord {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]PublishedRecord, len(b.records))
	copy(out, b.records)

	return out
}
