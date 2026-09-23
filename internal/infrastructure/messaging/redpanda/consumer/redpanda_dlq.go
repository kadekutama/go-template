// Package consumer: durable DLQ sinks. Unlike the in-memory test doubles in
// the fakes package, these write poison messages to the Redpanda dead-letter
// topics so alerts and operator replay see the same durable record a
// consumer group failed on (docs/domain-events.md §7).
package consumer

import (
	"context"
	"fmt"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/webhook"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// RedpandaMessageDLQ writes exhausted consumer messages to the durable
// LedgerEvents DLQ topic, partitioned per tenant so replay keeps ordering.
type RedpandaMessageDLQ struct {
	broker redpanda.Broker
	topics *redpanda.TopicRegistry
	logger log.Logger
}

// RedpandaMessageDLQParams carries constructor dependencies.
type RedpandaMessageDLQParams struct {
	Broker redpanda.Broker
	Topics *redpanda.TopicRegistry
	Logger log.Logger
}

// Compile-time sink conformance.
var _ DLQSink = (*RedpandaMessageDLQ)(nil)

// NewRedpandaMessageDLQ builds the durable sink; Broker and Topics are required.
func NewRedpandaMessageDLQ(params RedpandaMessageDLQParams) (*RedpandaMessageDLQ, error) {
	if params.Broker == nil {
		return nil, fmt.Errorf("dlq: broker is required")
	}

	if params.Topics == nil {
		return nil, fmt.Errorf("dlq: topic registry is required")
	}

	return &RedpandaMessageDLQ{broker: params.Broker, topics: params.Topics, logger: params.Logger}, nil
}

// messageEnvelope is the durable DLQ record for a consumer message.
type messageEnvelope struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Subject     string `json:"subject"`
	Redelivered int    `json:"redelivered"`
	PublishedAt string `json:"published_at"`
	Payload     []byte `json:"payload"`
}

// Record publishes the poison message to the DLQ topic (at-least-once; the
// broker write failing surfaces so the caller keeps the claim/ack decision).
func (s *RedpandaMessageDLQ) Record(ctx context.Context, msg appport.Message) error {
	if s == nil || s.broker == nil || s.topics == nil {
		return fmt.Errorf("dlq: not initialized")
	}

	dlqTopic, err := s.topics.DLQFor(s.topics.LedgerEventsTopic())
	if err != nil {
		return err
	}

	partition, err := redpanda.PartitionKey(msg.TenantID.String(), msg.ID)
	if err != nil {
		return err
	}

	body, err := jsonparser.Marshal(messageEnvelope{
		ID:          msg.ID,
		TenantID:    msg.TenantID.String(),
		Subject:     msg.Subject,
		Redelivered: msg.Redelivered,
		PublishedAt: msg.PublishedAt.UTC().Format(time.RFC3339Nano),
		Payload:     msg.Payload,
	})
	if err != nil {
		return err
	}

	if err := s.broker.Publish(ctx, dlqTopic, partition, map[string]string{
		"dlq_source": "consumer",
		"tenant_id":  msg.TenantID.String(),
	}, body); err != nil {
		return fmt.Errorf("dlq: publish message %s: %w", msg.ID, err)
	}

	return nil
}

// RedpandaWebhookDLQ writes exhausted webhook deliveries to the durable
// WebhookJobs DLQ topic.
type RedpandaWebhookDLQ struct {
	broker redpanda.Broker
	topics *redpanda.TopicRegistry
	logger log.Logger
}

// RedpandaWebhookDLQParams carries constructor dependencies.
type RedpandaWebhookDLQParams struct {
	Broker redpanda.Broker
	Topics *redpanda.TopicRegistry
	Logger log.Logger
}

// Compile-time sink conformance.
var _ webhook.DLQSink = (*RedpandaWebhookDLQ)(nil)

// NewRedpandaWebhookDLQ builds the durable sink; Broker and Topics are required.
func NewRedpandaWebhookDLQ(params RedpandaWebhookDLQParams) (*RedpandaWebhookDLQ, error) {
	if params.Broker == nil {
		return nil, fmt.Errorf("dlq: broker is required")
	}

	if params.Topics == nil {
		return nil, fmt.Errorf("dlq: topic registry is required")
	}

	return &RedpandaWebhookDLQ{broker: params.Broker, topics: params.Topics, logger: params.Logger}, nil
}

// webhookEnvelope is the durable DLQ record for a failed webhook delivery.
type webhookEnvelope struct {
	EndpointID string `json:"endpoint_id"`
	TenantID   string `json:"tenant_id"`
	Event      string `json:"event"`
	Attempts   int    `json:"attempts"`
	Payload    []byte `json:"payload"`
}

// Record publishes the exhausted delivery to the DLQ topic.
func (s *RedpandaWebhookDLQ) Record(ctx context.Context, msg webhook.DLQMessage) error {
	if s == nil || s.broker == nil || s.topics == nil {
		return fmt.Errorf("dlq: not initialized")
	}

	dlqTopic, err := s.topics.DLQFor(s.topics.WebhookJobsTopic())
	if err != nil {
		return err
	}

	partition, err := redpanda.PartitionKey(msg.Tenant, msg.EndpointID)
	if err != nil {
		return err
	}

	body, err := jsonparser.Marshal(webhookEnvelope{
		EndpointID: msg.EndpointID,
		TenantID:   msg.Tenant,
		Event:      msg.Event,
		Attempts:   msg.Attempts,
		Payload:    msg.Payload,
	})
	if err != nil {
		return err
	}

	if err := s.broker.Publish(ctx, dlqTopic, partition, map[string]string{
		"dlq_source": "webhook-dispatcher",
		"tenant_id":  msg.Tenant,
	}, body); err != nil {
		return fmt.Errorf("dlq: publish webhook %s: %w", msg.EndpointID, err)
	}

	return nil
}
