package redpanda_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
)

func TestRegistry(t *testing.T) {
	t.Parallel()

	baseConfig := redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	}

	type testCase struct {
		name           string
		cfg            redpanda.TopicsConfig
		expectedResult []redpanda.TopicSpec
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "zero config rejected",
			cfg:            redpanda.TopicsConfig{},
			expectedResult: nil,
			expectedError:  errors.New("redpanda: ledger events topic is required"),
		},
		{
			name: "valid registry",
			cfg:  baseConfig,
			expectedResult: []redpanda.TopicSpec{
				{
					Name:          "ledger.events.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "ledger.events.v1.dlq",
				},
				{
					Name:          "outbox.facts.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "outbox.facts.v1.dlq",
				},
				{
					Name:          "webhook.jobs.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "webhook.jobs.v1.dlq",
				},
				{
					Name:          "audit.streams.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "audit.streams.v1.dlq",
				},
			},
			expectedError: nil,
		},
		{
			name: "custom names and sizing",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.LedgerEvents = "custom.ledger.v1"
				c.Partitions = 6
				c.RetentionHrs = 48
				return c
			}(),
			expectedResult: []redpanda.TopicSpec{
				{
					Name:          "custom.ledger.v1",
					Partitions:    6,
					RetentionHrs:  48,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "custom.ledger.v1.dlq",
				},
				{
					Name:          "outbox.facts.v1",
					Partitions:    6,
					RetentionHrs:  48,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "outbox.facts.v1.dlq",
				},
				{
					Name:          "webhook.jobs.v1",
					Partitions:    6,
					RetentionHrs:  48,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "webhook.jobs.v1.dlq",
				},
				{
					Name:          "audit.streams.v1",
					Partitions:    6,
					RetentionHrs:  48,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "audit.streams.v1.dlq",
				},
			},
			expectedError: nil,
		},
		{
			name: "duplicate names rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.OutboxFacts = c.LedgerEvents
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: duplicate topic name \"ledger.events.v1\""),
		},
		{
			name: "missing partition keys rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.PartitionKeys = ""
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: partition keys is required"),
		},
		{
			name: "missing dlq suffix rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.DLQSuffix = ""
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: dlq suffix is required"),
		},
		{
			name: "padded topic stored trimmed",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.LedgerEvents = "  ledger.events.v1  "
				return c
			}(),
			expectedResult: []redpanda.TopicSpec{
				{
					Name:          "ledger.events.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "ledger.events.v1.dlq",
				},
				{
					Name:          "outbox.facts.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "outbox.facts.v1.dlq",
				},
				{
					Name:          "webhook.jobs.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "webhook.jobs.v1.dlq",
				},
				{
					Name:          "audit.streams.v1",
					Partitions:    12,
					RetentionHrs:  720,
					PartitionKeys: "tenant_id:account_id",
					DLQ:           "audit.streams.v1.dlq",
				},
			},
			expectedError: nil,
		},
		{
			name: "padded duplicate rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.OutboxFacts = "  ledger.events.v1  "
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: duplicate topic name \"ledger.events.v1\""),
		},
		{
			name: "whitespace topic rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.LedgerEvents = "   "
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: ledger events topic is required"),
		},
		{
			name: "zero partitions rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.Partitions = 0
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: partitions must be positive"),
		},
		{
			name: "negative partitions rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.Partitions = -2
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: partitions must be positive"),
		},
		{
			name: "zero retention rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.RetentionHrs = 0
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: retention hours must be positive"),
		},
		{
			name: "whitespace partition keys rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.PartitionKeys = "   "
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: partition keys is required"),
		},
		{
			name: "whitespace dlq suffix rejected",
			cfg: func() redpanda.TopicsConfig {
				c := baseConfig
				c.DLQSuffix = "   "
				return c
			}(),
			expectedResult: nil,
			expectedError:  errors.New("redpanda: dlq suffix is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			registry, err := redpanda.NewTopicRegistry(tc.cfg)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, registry)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, registry)
			assert.Equal(t, tc.expectedResult, registry.Registry())
		})
	}
}

func TestIsKnownTopic(t *testing.T) {
	t.Parallel()

	registry, err := redpanda.NewTopicRegistry(redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	})
	require.NoError(t, err)

	type testCase struct {
		name           string
		topicName      string
		expectedResult bool
	}

	testCases := []testCase{
		{
			name:           "ledger events topic",
			topicName:      "ledger.events.v1",
			expectedResult: true,
		},
		{
			name:           "outbox facts topic",
			topicName:      "outbox.facts.v1",
			expectedResult: true,
		},
		{
			name:           "webhook jobs topic",
			topicName:      "webhook.jobs.v1",
			expectedResult: true,
		},
		{
			name:           "audit streams topic",
			topicName:      "audit.streams.v1",
			expectedResult: true,
		},
		{
			name:           "unknown topic",
			topicName:      "unknown.topic",
			expectedResult: false,
		},
		{
			name:           "empty string",
			topicName:      "",
			expectedResult: false,
		},
		{
			name:           "whitespace only",
			topicName:      "   ",
			expectedResult: false,
		},
		{
			name:           "padded known topic matches",
			topicName:      "  ledger.events.v1  ",
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, registry.IsKnownTopic(tc.topicName))
		})
	}
}

func TestAllTopics(t *testing.T) {
	t.Parallel()

	registry, err := redpanda.NewTopicRegistry(redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	})
	require.NoError(t, err)

	type testCase struct {
		name           string
		mutateIndex    int
		mutateValue    string
		expectedLength int
		expectedFirst  string
	}

	testCases := []testCase{
		{
			name:           "copy isolated from mutation",
			mutateIndex:    0,
			mutateValue:    "mutated.topic",
			expectedLength: 4,
			expectedFirst:  "ledger.events.v1",
		},
		{
			name:           "copy isolated from empty string mutation",
			mutateIndex:    0,
			mutateValue:    "",
			expectedLength: 4,
			expectedFirst:  "ledger.events.v1",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			topics := registry.AllTopics()
			require.Len(t, topics, tc.expectedLength)
			assert.Equal(t, tc.expectedFirst, topics[0])

			// Mutate returned copy
			topics[tc.mutateIndex] = tc.mutateValue

			// Ensure subsequent call is uncorrupted
			second := registry.AllTopics()
			assert.Equal(t, tc.expectedFirst, second[0])
		})
	}
}

func TestTopicRegistryGetters(t *testing.T) {
	t.Parallel()

	cfg := redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	}

	registry, err := redpanda.NewTopicRegistry(cfg)
	require.NoError(t, err)

	assert.Equal(t, "ledger.events.v1", registry.LedgerEventsTopic())
	assert.Equal(t, "outbox.facts.v1", registry.OutboxFactsTopic())
	assert.Equal(t, "webhook.jobs.v1", registry.WebhookJobsTopic())
	assert.Equal(t, "audit.streams.v1", registry.AuditStreamsTopic())
	assert.Equal(t, "tenant_id:account_id", registry.PartitionKeys())
	assert.Equal(t, ".dlq", registry.DLQSuffix())
}

func TestPartitionKey(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		account        string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "tenant and account",
			tenant:         "t1",
			account:        "a1",
			expectedResult: "t1:a1",
			expectedError:  nil,
		},
		{
			name:           "trims surrounding whitespace",
			tenant:         "  t1  ",
			account:        "  a1  ",
			expectedResult: "t1:a1",
			expectedError:  nil,
		},
		{
			name:           "empty tenant rejected",
			tenant:         "",
			account:        "a1",
			expectedResult: "",
			expectedError:  errors.New("redpanda: tenant and account are required"),
		},
		{
			name:           "whitespace tenant rejected",
			tenant:         "   ",
			account:        "a1",
			expectedResult: "",
			expectedError:  errors.New("redpanda: tenant and account are required"),
		},
		{
			name:           "empty account rejected",
			tenant:         "t1",
			account:        "",
			expectedResult: "",
			expectedError:  errors.New("redpanda: tenant and account are required"),
		},
		{
			name:           "whitespace account rejected",
			tenant:         "t1",
			account:        "   ",
			expectedResult: "",
			expectedError:  errors.New("redpanda: tenant and account are required"),
		},
		{
			name:           "colon in tenant rejected",
			tenant:         "t:1",
			account:        "a1",
			expectedResult: "",
			expectedError:  errors.New("redpanda: partition segments must not contain ':'"),
		},
		{
			name:           "colon in account rejected",
			tenant:         "t1",
			account:        "a:1",
			expectedResult: "",
			expectedError:  errors.New("redpanda: partition segments must not contain ':'"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := redpanda.PartitionKey(tc.tenant, tc.account)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestDLQFor(t *testing.T) {
	t.Parallel()

	registry, err := redpanda.NewTopicRegistry(redpanda.TopicsConfig{
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	})
	require.NoError(t, err)

	type testCase struct {
		name           string
		topicOrGroup   string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "known topic maps to dlq",
			topicOrGroup:   "ledger.events.v1",
			expectedResult: "ledger.events.v1.dlq",
			expectedError:  nil,
		},
		{
			name:           "group maps to namespaced dlq",
			topicOrGroup:   "webhook-dispatcher",
			expectedResult: "webhook-dispatcher.dlq",
			expectedError:  nil,
		},
		{
			name:           "dlq input stable",
			topicOrGroup:   "x.dlq",
			expectedResult: "x.dlq",
			expectedError:  nil,
		},
		{
			name:           "blank rejected",
			topicOrGroup:   "",
			expectedResult: "",
			expectedError:  errors.New("redpanda: topic or group is required"),
		},
		{
			name:           "whitespace rejected",
			topicOrGroup:   "   ",
			expectedResult: "",
			expectedError:  errors.New("redpanda: topic or group is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := registry.DLQFor(tc.topicOrGroup)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
