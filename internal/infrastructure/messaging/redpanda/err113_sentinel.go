package redpanda

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrRedpandaAtLeastOneSeedIsRequired                          = errors.New("redpanda: at least one seed is required")
	ErrRedpandaInvalidParams1ViolationSDialTimeoutRuleGtOnValue1 = errors.New("redpanda: invalid params (1 violation(s)): DialTimeout: rule \"gt\" on value -1s: validation violation")
	ErrRedpandaInvalidParams1ViolationSDialTimeoutRuleRequiredOn = errors.New("redpanda: invalid params (1 violation(s)): DialTimeout: rule \"required\" on value 0s: validation violation")
	ErrRedpandaInvalidParams1ViolationSMetadataTimeoutRuleRequir = errors.New("redpanda: invalid params (1 violation(s)): MetadataTimeout: rule \"required\" on value 0s: validation violation")
	ErrRedpandaSaslMechanismIsRequiredWhenSaslCredentialsAreProv = errors.New("redpanda: sasl mechanism is required when sasl credentials are provided")
	ErrRedpandaProducerIsNotInitialized                          = errors.New("redpanda: producer is not initialized")
	ErrRedpandaTopicIsRequired                                   = errors.New("redpanda: topic is required")
	ErrRedpandaPublishLedgerEventsV1ClientClosed                 = errors.New("redpanda: publish ledger.events.v1: client closed")
	ErrRedpandaLedgerEventsTopicIsRequired                       = errors.New("redpanda: ledger events topic is required")
	ErrRedpandaDuplicateTopicNameLedgerEventsV1                  = errors.New("redpanda: duplicate topic name \"ledger.events.v1\": redpanda: duplicate topic name")
	ErrRedpandaPartitionKeysIsRequired                           = errors.New("redpanda: partition keys is required")
	ErrRedpandaDlqSuffixIsRequired                               = errors.New("redpanda: dlq suffix is required")
	ErrRedpandaPartitionsMustBePositive                          = errors.New("redpanda: partitions must be positive")
	ErrRedpandaRetentionHoursMustBePositive                      = errors.New("redpanda: retention hours must be positive")
	ErrRedpandaTenantAndAccountAreRequired                       = errors.New("redpanda: tenant and account are required")
	ErrRedpandaPartitionSegmentsMustNotContain                   = errors.New("redpanda: partition segments must not contain ':'")
	ErrRedpandaTopicOrGroupIsRequired                            = errors.New("redpanda: topic or group is required")
	ErrRedpandaOutboxFactsTopicIsRequired                        = errors.New("redpanda: outbox facts topic is required")
	ErrRedpandaWebhookJobsTopicIsRequired                        = errors.New("redpanda: webhook jobs topic is required")
	ErrRedpandaAuditStreamsTopicIsRequired                       = errors.New("redpanda: audit streams topic is required")
	ErrRedpandaDialTimeoutMustBePositive                         = errors.New("redpanda: dial timeout must be positive")
	ErrRedpandaMetadataTimeoutMustBePositive                     = errors.New("redpanda: metadata timeout must be positive")
	ErrRedpandaUnknownSaslMechanism                              = errors.New("redpanda: unknown sasl mechanism")
	ErrRedpandaDuplicateTopicName                                = errors.New("redpanda: duplicate topic name")
)
