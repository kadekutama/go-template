package fakes

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
)

// ReceiptStore is an in-memory consumer inbox test double: it dedupes within
// one process and is never a durable store. Production uses the Valkey hint
// plus the PostgreSQL inbox (E14 binding).
type ReceiptStore struct {
	mu   sync.Mutex
	seen map[string]struct{}
}

// NewReceiptStore builds the double.
func NewReceiptStore() *ReceiptStore {
	return &ReceiptStore{seen: make(map[string]struct{})}
}

// Claim inserts (consumer, eventID); the second claim is a duplicate.
func (s *ReceiptStore) Claim(_ context.Context, consumer, eventID string) (bool, error) {
	if strings.TrimSpace(consumer) == "" || strings.TrimSpace(eventID) == "" {
		return false, fmt.Errorf("consumer: consumer and event id are required")
	}

	key := strings.TrimSpace(consumer) + ":" + strings.TrimSpace(eventID)

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.seen[key]; ok {
		return true, nil
	}

	s.seen[key] = struct{}{}

	return false, nil
}

// Release drops the receipt so a redelivery re-runs the handler. Missing
// receipts (and repeated releases) succeed.
func (s *ReceiptStore) Release(_ context.Context, consumer, eventID string) error {
	if strings.TrimSpace(consumer) == "" || strings.TrimSpace(eventID) == "" {
		return fmt.Errorf("consumer: consumer and event id are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.seen, strings.TrimSpace(consumer)+":"+strings.TrimSpace(eventID))

	return nil
}

// MessageDLQ records exhausted consumer messages in process order for
// assertions and replay tests.
type MessageDLQ struct {
	mu   sync.Mutex
	msgs []appport.Message
}

// NewMessageDLQ builds the double.
func NewMessageDLQ() *MessageDLQ {
	return &MessageDLQ{}
}

// Record appends the poison message with its redelivery count preserved.
func (d *MessageDLQ) Record(_ context.Context, msg appport.Message) error {
	if strings.TrimSpace(msg.ID) == "" {
		return fmt.Errorf("dlq: message id is required")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	d.msgs = append(d.msgs, msg)

	return nil
}

// List returns a copy of recorded messages in arrival order.
func (d *MessageDLQ) List() []appport.Message {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]appport.Message, len(d.msgs))
	copy(out, d.msgs)

	return out
}

// Len reports the recorded count.
func (d *MessageDLQ) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.msgs)
}

// WebhookDLQ records exhausted webhook deliveries for assertions.
type WebhookDLQ struct {
	mu   sync.Mutex
	msgs []webhook.DLQMessage
}

// NewWebhookDLQ builds the double.
func NewWebhookDLQ() *WebhookDLQ {
	return &WebhookDLQ{}
}

// Record appends the exhausted delivery with its attempt count preserved.
func (d *WebhookDLQ) Record(_ context.Context, msg webhook.DLQMessage) error {
	if strings.TrimSpace(msg.EndpointID) == "" {
		return errors.New("webhook: endpoint id is required")
	}

	if msg.Payload == nil {
		return errors.New("webhook: payload is required")
	}

	cpPayload := make([]byte, len(msg.Payload))
	copy(cpPayload, msg.Payload)
	msg.Payload = cpPayload

	d.mu.Lock()
	defer d.mu.Unlock()

	d.msgs = append(d.msgs, msg)

	return nil
}

// List returns a copy of recorded messages in arrival order.
func (d *WebhookDLQ) List() []webhook.DLQMessage {
	d.mu.Lock()
	defer d.mu.Unlock()

	out := make([]webhook.DLQMessage, len(d.msgs))
	for i, m := range d.msgs {
		if m.Payload != nil {
			cpPayload := make([]byte, len(m.Payload))
			copy(cpPayload, m.Payload)
			m.Payload = cpPayload
		}
		out[i] = m
	}

	return out
}

// Len reports the recorded count.
func (d *WebhookDLQ) Len() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.msgs)
}
