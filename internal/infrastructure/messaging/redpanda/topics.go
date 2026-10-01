// Package redpanda is the Redpanda v26.2 durable-log topology and data plane
// (E08-T03/T04): topic registry, thin client config, outbox-driven publisher,
// and the idempotent consumer framework with DLQ. Partitioning preserves
// per-account FIFO on tenant_id:account_id. Consumers apply read-side
// effects only; replay never re-runs ledger commands.
package redpanda

import (
	"fmt"
	"strings"
)

// TopicsConfig carries the configurable durable-topic names and topology.
// All fields must be explicitly configured and non-empty.
type TopicsConfig struct {
	LedgerEvents  string
	OutboxFacts   string
	WebhookJobs   string
	AuditStreams  string
	Partitions    int
	RetentionHrs  int
	PartitionKeys string
	DLQSuffix     string
}

// Validate ensures all required topic names, sizing, and topology settings are valid.
func (c TopicsConfig) Validate() error {
	if strings.TrimSpace(c.LedgerEvents) == "" {
		return fmt.Errorf("redpanda: ledger events topic is required")
	}
	if strings.TrimSpace(c.OutboxFacts) == "" {
		return fmt.Errorf("redpanda: outbox facts topic is required")
	}
	if strings.TrimSpace(c.WebhookJobs) == "" {
		return fmt.Errorf("redpanda: webhook jobs topic is required")
	}
	if strings.TrimSpace(c.AuditStreams) == "" {
		return fmt.Errorf("redpanda: audit streams topic is required")
	}
	if c.Partitions <= 0 {
		return fmt.Errorf("redpanda: partitions must be positive")
	}
	if c.RetentionHrs <= 0 {
		return fmt.Errorf("redpanda: retention hours must be positive")
	}
	if strings.TrimSpace(c.PartitionKeys) == "" {
		return fmt.Errorf("redpanda: partition keys is required")
	}
	if strings.TrimSpace(c.DLQSuffix) == "" {
		return fmt.Errorf("redpanda: dlq suffix is required")
	}
	return nil
}

// TopicRegistry is an immutable, config-driven view of the durable topics.
// Build one with NewTopicRegistry; the zero value is not usable.
type TopicRegistry struct {
	names []string
	known map[string]struct{}
	cfg   TopicsConfig
}

// NewTopicRegistry builds the registry, rejecting empty, non-positive, or duplicate topic
// configuration so a bad configuration fails fast at wiring time. Inputs are
// trimmed at ingestion so stored names, duplicate detection, and lookups all
// operate on the same normalized values (an untrimmed name would otherwise
// provision with whitespace yet miss trimmed lookups).
func NewTopicRegistry(cfg TopicsConfig) (*TopicRegistry, error) {
	cfg.LedgerEvents = strings.TrimSpace(cfg.LedgerEvents)
	cfg.OutboxFacts = strings.TrimSpace(cfg.OutboxFacts)
	cfg.WebhookJobs = strings.TrimSpace(cfg.WebhookJobs)
	cfg.AuditStreams = strings.TrimSpace(cfg.AuditStreams)
	cfg.PartitionKeys = strings.TrimSpace(cfg.PartitionKeys)
	cfg.DLQSuffix = strings.TrimSpace(cfg.DLQSuffix)

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	names := []string{cfg.LedgerEvents, cfg.OutboxFacts, cfg.WebhookJobs, cfg.AuditStreams}

	known := make(map[string]struct{}, len(names))

	for _, name := range names {
		if _, dup := known[name]; dup {
			return nil, fmt.Errorf("redpanda: duplicate topic name %q", name)
		}

		known[name] = struct{}{}
	}

	return &TopicRegistry{names: names, known: known, cfg: cfg}, nil
}

// Registry returns the durable topic specs with per-group DLQ names.
func (r *TopicRegistry) Registry() []TopicSpec {
	specs := make([]TopicSpec, 0, len(r.names))

	for _, name := range r.names {
		specs = append(specs, TopicSpec{
			Name:          name,
			Partitions:    r.cfg.Partitions,
			RetentionHrs:  r.cfg.RetentionHrs,
			PartitionKeys: r.cfg.PartitionKeys,
			DLQ:           name + r.cfg.DLQSuffix,
		})
	}

	return specs
}

// AllTopics returns a copy of every durable topic in delivery order.
func (r *TopicRegistry) AllTopics() []string {
	out := make([]string, len(r.names))
	copy(out, r.names)

	return out
}

// LedgerEventsTopic returns the configured ledger-events topic.
func (r *TopicRegistry) LedgerEventsTopic() string { return r.cfg.LedgerEvents }

// OutboxFactsTopic returns the configured outbox-facts topic (relay target).
func (r *TopicRegistry) OutboxFactsTopic() string { return r.cfg.OutboxFacts }

// WebhookJobsTopic returns the configured webhook-jobs topic.
func (r *TopicRegistry) WebhookJobsTopic() string { return r.cfg.WebhookJobs }

// AuditStreamsTopic returns the configured audit-streams topic.
func (r *TopicRegistry) AuditStreamsTopic() string { return r.cfg.AuditStreams }

// PartitionKeys returns the configured canonical partition-key contract.
func (r *TopicRegistry) PartitionKeys() string { return r.cfg.PartitionKeys }

// DLQSuffix returns the configured dead-letter suffix.
func (r *TopicRegistry) DLQSuffix() string { return r.cfg.DLQSuffix }

// IsKnownTopic reports whether name is a registered durable topic.
// Input is trimmed so callers need not normalize before asking.
func (r *TopicRegistry) IsKnownTopic(name string) bool {
	_, ok := r.known[strings.TrimSpace(name)]

	return ok
}

// DLQFor returns the dead-letter topic for a durable topic or consumer
// group. Unknown names map to a namespaced DLQ instead of dropping; an
// already-qualified DLQ name passes through unchanged.
func (r *TopicRegistry) DLQFor(topicOrGroup string) (string, error) {
	trimmed := strings.TrimSpace(topicOrGroup)
	if trimmed == "" {
		return "", fmt.Errorf("redpanda: topic or group is required")
	}

	// Known topics and unqualified names get the canonical DLQ suffix;
	// an already-qualified DLQ name passes through unchanged.
	if r.IsKnownTopic(trimmed) || !strings.HasSuffix(trimmed, r.cfg.DLQSuffix) {
		return trimmed + r.cfg.DLQSuffix, nil
	}

	return trimmed, nil
}

// TopicSpec describes one durable topic for provisioning and review.
type TopicSpec struct {
	Name          string
	Partitions    int
	RetentionHrs  int
	PartitionKeys string
	DLQ           string
}

// PartitionKey preserves per-account FIFO. Empty segments fail typed.
func PartitionKey(tenant, account string) (string, error) {
	trimmedTenant := strings.TrimSpace(tenant)
	trimmedAccount := strings.TrimSpace(account)

	if trimmedTenant == "" || trimmedAccount == "" {
		return "", fmt.Errorf("redpanda: tenant and account are required")
	}

	if strings.Contains(trimmedTenant, ":") || strings.Contains(trimmedAccount, ":") {
		return "", fmt.Errorf("redpanda: partition segments must not contain ':'")
	}

	return trimmedTenant + ":" + trimmedAccount, nil
}
