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

// Fallback durable topic names backing the event log (ADR-014). They are
// unexported on purpose: runtime names come from configuration through
// TopicsConfig, so no caller hardcodes a topic string. Deployments override
// them per environment; the DLQ suffix convention stays fixed (<topic>.dlq).
const (
	defaultLedgerEventsTopic = "ledger.events.v1"
	defaultOutboxFactsTopic  = "outbox.facts.v1"
	defaultWebhookJobsTopic  = "webhook.jobs.v1"
	defaultAuditStreamsTopic = "audit.streams.v1"
)

// DefaultPartitions sizes new topics; retention is 30d with tiered S3
// (operated in E17; this registry records the contract for tests).
const (
	DefaultPartitions   = 12
	DefaultRetentionHrs = 24 * 30
)

// PartitionKeys is the canonical partition-key contract.
const PartitionKeys = "tenant_id:account_id"

// DLQSuffix is appended to every topic/group for dead-lettering.
const DLQSuffix = ".dlq"

// TopicsConfig carries the configurable durable-topic names. Zero values
// fall back to the ADR-014 defaults, so committed config files may omit the
// section until an environment needs different names.
type TopicsConfig struct {
	LedgerEvents string
	OutboxFacts  string
	WebhookJobs  string
	AuditStreams string
	Partitions   int
	RetentionHrs int
}

// DefaultTopicsConfig returns the ADR-014 topic defaults.
func DefaultTopicsConfig() TopicsConfig {
	return TopicsConfig{
		LedgerEvents: defaultLedgerEventsTopic,
		OutboxFacts:  defaultOutboxFactsTopic,
		WebhookJobs:  defaultWebhookJobsTopic,
		AuditStreams: defaultAuditStreamsTopic,
		Partitions:   DefaultPartitions,
		RetentionHrs: DefaultRetentionHrs,
	}
}

// WithDefaults fills zero values with the contract defaults, so partial
// configuration overlays ADR-014 instead of yielding empty topic names.
func (c TopicsConfig) WithDefaults() TopicsConfig {
	out := c

	if out.LedgerEvents == "" {
		out.LedgerEvents = defaultLedgerEventsTopic
	}

	if out.OutboxFacts == "" {
		out.OutboxFacts = defaultOutboxFactsTopic
	}

	if out.WebhookJobs == "" {
		out.WebhookJobs = defaultWebhookJobsTopic
	}

	if out.AuditStreams == "" {
		out.AuditStreams = defaultAuditStreamsTopic
	}

	if out.Partitions <= 0 {
		out.Partitions = DefaultPartitions
	}

	if out.RetentionHrs <= 0 {
		out.RetentionHrs = DefaultRetentionHrs
	}

	return out
}

// TopicRegistry is an immutable, config-driven view of the durable topics.
// Build one with NewTopicRegistry; the zero value is not usable.
type TopicRegistry struct {
	names []string
	known map[string]struct{}
	cfg   TopicsConfig
}

// NewTopicRegistry builds the registry, rejecting empty or duplicate topic
// names so a bad configuration fails fast at wiring time.
func NewTopicRegistry(cfg TopicsConfig) (*TopicRegistry, error) {
	effective := cfg.WithDefaults()

	names := []string{effective.LedgerEvents, effective.OutboxFacts, effective.WebhookJobs, effective.AuditStreams}

	known := make(map[string]struct{}, len(names))

	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("redpanda: topic names are required")
		}

		if _, dup := known[name]; dup {
			return nil, fmt.Errorf("redpanda: duplicate topic name %q", name)
		}

		known[name] = struct{}{}
	}

	return &TopicRegistry{names: names, known: known, cfg: effective}, nil
}

// Registry returns the durable topic specs with per-group DLQ names.
func (r *TopicRegistry) Registry() []TopicSpec {
	specs := make([]TopicSpec, 0, len(r.names))

	for _, name := range r.names {
		specs = append(specs, TopicSpec{
			Name:          name,
			Partitions:    r.cfg.Partitions,
			RetentionHrs:  r.cfg.RetentionHrs,
			PartitionKeys: PartitionKeys,
			DLQ:           name + DLQSuffix,
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

// IsKnownTopic reports whether name is a registered durable topic.
func (r *TopicRegistry) IsKnownTopic(name string) bool {
	_, ok := r.known[name]

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

	// Known topics and unqualified names get the canonical .dlq suffix;
	// an already-qualified DLQ name passes through unchanged.
	if r.IsKnownTopic(trimmed) || !strings.HasSuffix(trimmed, DLQSuffix) {
		return trimmed + DLQSuffix, nil
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

// DefaultTopicRegistry is the package-level registry built from the default
// topic configuration; wiring that reads configuration builds its own.
var defaultTopicRegistry = mustTopicRegistry(DefaultTopicsConfig())

// DefaultTopicRegistry returns the ADR-014 default registry for callers that
// have no configuration override (tests, local tooling).
func DefaultTopicRegistry() *TopicRegistry {
	return defaultTopicRegistry
}

func mustTopicRegistry(cfg TopicsConfig) *TopicRegistry {
	registry, err := NewTopicRegistry(cfg)
	if err != nil {
		panic(fmt.Sprintf("redpanda: default topic registry: %v", err))
	}

	return registry
}

// AllTopics returns a copy of every durable topic in delivery order.
func AllTopics() []string {
	return defaultTopicRegistry.AllTopics()
}

// Registry returns the durable topic specs with per-group DLQ names.
func Registry() []TopicSpec {
	return defaultTopicRegistry.Registry()
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

// DLQFor returns the dead-letter topic for a durable topic or consumer
// group against the default registry. Unknown names map to a namespaced
// DLQ instead of dropping.
func DLQFor(topicOrGroup string) (string, error) {
	return defaultTopicRegistry.DLQFor(topicOrGroup)
}

// IsKnownTopic reports whether name is a registered durable topic against
// the default registry.
func IsKnownTopic(name string) bool {
	return defaultTopicRegistry.IsKnownTopic(name)
}
