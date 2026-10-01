// Package doubles holds tiny mock-programming helpers shared by messaging
// test suites (unit + integration). They contain no business logic and model
// no production behavior: a dedup set for receipt-claim doubles, a publish
// snapshot that deep-copies captured broker output, and an append-only
// message log for DLQ captures. One copy lives here so the four receipt,
// four broker-capture, and two DLQ copy sites cannot drift.
package doubles

import (
	"sync"
	"sync/atomic"
)

// DedupSet models single-use claim/release semantics for receipt-store
// doubles: the first Claim reports false (live), repeats report true
// (duplicate) until Release clears the key.
type DedupSet struct {
	seen sync.Map
}

// Claim records consumer:eventID and reports whether it was already present.
func (d *DedupSet) Claim(consumer, eventID string) bool {
	_, loaded := d.seen.LoadOrStore(consumer+":"+eventID, struct{}{})
	return loaded
}

// Release clears consumer:eventID so the next Claim reports live.
func (d *DedupSet) Release(consumer, eventID string) {
	d.seen.Delete(consumer + ":" + eventID)
}

// PublishRecord is one captured broker publish with defensively copied
// headers and payload: the producer may reuse its buffers after Publish
// returns, so captures must never alias caller memory.
type PublishRecord struct {
	Topic   string
	Key     string
	Headers map[string]string
	Payload []byte
}

// SnapshotPublish captures one publish with deep-copied headers and payload.
func SnapshotPublish(topic, key string, headers map[string]string, payload []byte) PublishRecord {
	headersCopy := make(map[string]string, len(headers))
	for k, v := range headers {
		headersCopy[k] = v
	}
	return PublishRecord{
		Topic:   topic,
		Key:     key,
		Headers: headersCopy,
		Payload: append([]byte(nil), payload...),
	}
}

// MessageLog is an append-only, concurrency-safe capture log for DLQ
// doubles. Reads via All return a copy taken at call time.
type MessageLog[T any] struct {
	messages atomic.Pointer[[]T]
}

// Add appends one message via copy-on-write; contention retries.
func (l *MessageLog[T]) Add(msg T) {
	for {
		old := l.messages.Load()
		var next []T
		if old != nil {
			next = append(append([]T(nil), *old...), msg)
		} else {
			next = []T{msg}
		}
		if l.messages.CompareAndSwap(old, &next) {
			return
		}
	}
}

// All returns a copy of every recorded message.
func (l *MessageLog[T]) All() []T {
	loaded := l.messages.Load()
	if loaded == nil {
		return nil
	}
	return append([]T(nil), *loaded...)
}
