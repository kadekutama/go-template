// Package publisher is the outbox-driven Redpanda relay (E08-T04): it
// implements port.EventPublisher over already-committed outbox facts with
// at-least-once delivery. Encoding uses pkg/jsonparser; partitioning
// preserves per-aggregate FIFO on tenant:aggregate. The E07 outbox poller
// calls Publish; consumers dedupe on fact identity (E08-T04 framework).
package publisher

import (
	"context"
	"fmt"
	"strings"
	"time"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// PublisherParams carries constructor dependencies (Parameter Object pattern).
// Topic <= "" selects the ADR-014 outbox-facts default; wiring passes the
// configured registry's OutboxFactsTopic().
type PublisherParams struct {
	Broker redpanda.Broker
	Topic  string
	Logger log.Logger
}

// Publisher relays committed facts. It is safe for concurrent use when the
// broker is.
type Publisher struct {
	broker redpanda.Broker
	topic  string
	logger log.Logger
}

// Compile-time port conformance.
var _ appport.EventPublisher = (*Publisher)(nil)

// NewPublisher builds the relay; Broker must be non-nil.
func NewPublisher(params PublisherParams) (*Publisher, error) {
	if params.Broker == nil {
		return nil, fmt.Errorf("publisher: broker is required")
	}

	topic := strings.TrimSpace(params.Topic)
	if topic == "" {
		topic = redpanda.DefaultTopicsConfig().OutboxFacts
	}

	return &Publisher{broker: params.Broker, topic: topic, logger: params.Logger}, nil
}

// envelope is the on-wire fact shape (headers carry identity + trace).
// Trace/request IDs ride both the envelope body and the transport headers
// so consumers restore them onto the handler context (E01-T05 correlation).
type envelope struct {
	TenantID    string `json:"tenant_id"`
	LedgerID    string `json:"ledger_id"`
	EventType   string `json:"event_type"`
	AggregateID string `json:"aggregate_id"`
	OccurredAt  string `json:"occurred_at"`
	TraceID     string `json:"trace_id"`
	RequestID   string `json:"request_id"`
	Payload     []byte `json:"payload"`
}

// Publish relays already-committed facts. Empty batches succeed without I/O.
// Unknown publish outcome is an error: the E07 poller row stays claimable.
func (p *Publisher) Publish(ctx context.Context, facts ...appport.OutboxFact) error {
	if p == nil || p.broker == nil {
		return fmt.Errorf("publisher: not initialized")
	}

	if len(facts) == 0 {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("publisher: publish: %w", err)
	}

	for idx := range facts {
		if err := p.publishOne(ctx, facts[idx]); err != nil {
			return err
		}
	}

	return nil
}

func (p *Publisher) publishOne(ctx context.Context, fact appport.OutboxFact) error {
	tenant := strings.TrimSpace(fact.TenantID.String())
	eventType := strings.TrimSpace(fact.EventType)
	aggregate := strings.TrimSpace(fact.AggregateID)

	if tenant == "" {
		return fmt.Errorf("publisher: tenant is required")
	}

	if eventType == "" {
		return fmt.Errorf("publisher: event type is required")
	}

	if aggregate == "" {
		return fmt.Errorf("publisher: aggregate id is required")
	}

	if fact.Payload == nil {
		return fmt.Errorf("publisher: payload is required")
	}

	occurred := fact.OccurredAt
	if occurred.IsZero() {
		occurred = time.Now().UTC()
	}

	correlation := log.FromContext(ctx)

	body, err := jsonparser.Marshal(envelope{
		TenantID:    tenant,
		LedgerID:    strings.TrimSpace(fact.LedgerID.String()),
		EventType:   eventType,
		AggregateID: aggregate,
		OccurredAt:  occurred.UTC().Format(time.RFC3339Nano),
		TraceID:     correlation[log.FieldTraceID],
		RequestID:   correlation[log.FieldRequestID],
		Payload:     fact.Payload,
	})
	if err != nil {
		return err
	}

	partition, err := redpanda.PartitionKey(tenant, aggregate)
	if err != nil {
		return err
	}

	headers := map[string]string{
		"event_type":   eventType,
		"tenant_id":    tenant,
		"aggregate_id": aggregate,
	}

	if traceID := correlation[log.FieldTraceID]; traceID != "" {
		headers["trace_id"] = traceID
	}

	if requestID := correlation[log.FieldRequestID]; requestID != "" {
		headers["request_id"] = requestID
	}

	if err := p.broker.Publish(ctx, p.topic, partition, headers, body); err != nil {
		return fmt.Errorf("publisher: publish %s: %w", eventType, err)
	}

	return nil
}
