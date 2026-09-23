package config_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/config"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
)

func TestCacheTTLsConfigHybridTTLs(t *testing.T) {
	t.Parallel()

	defaults := hybrid.DefaultTTLs()

	type testCase struct {
		name           string
		ttls           config.CacheTTLsConfig
		expectedResult hybrid.TTLSet
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "zero config keeps contract defaults",
			ttls:           config.CacheTTLsConfig{},
			expectedResult: defaults,
			expectedError:  nil,
		},
		{
			name: "overrides apply and unspecified keep defaults",
			ttls: config.CacheTTLsConfig{
				BalanceSec:    120,
				L1PopulateSec: 10,
			},
			expectedResult: func() hybrid.TTLSet {
				ttl := defaults
				ttl.Balance = 120 * time.Second
				ttl.L1Populate = 10 * time.Second

				return ttl
			}(),
			expectedError: nil,
		},
		{
			name: "negative override rejected",
			ttls: config.CacheTTLsConfig{
				BalanceSec: -1,
			},
			expectedResult: hybrid.TTLSet{},
			expectedError:  errors.New("cache: ttl balance must not be negative"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ttl, err := tc.ttls.HybridTTLs()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, ttl)
		})
	}
}

func TestRedpandaConfigTopics(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		redpanda       config.RedpandaConfig
		expectedResult redpanda.TopicsConfig
	}

	testCases := []testCase{
		{
			name:           "zero config keeps defaults",
			redpanda:       config.RedpandaConfig{},
			expectedResult: redpanda.DefaultTopicsConfig(),
		},
		{
			name: "overrides map through",
			redpanda: config.RedpandaConfig{
				LedgerEvents: "custom.events.v2",
				Partitions:   24,
			},
			expectedResult: redpanda.TopicsConfig{
				LedgerEvents: "custom.events.v2",
				OutboxFacts:  redpanda.DefaultTopicsConfig().OutboxFacts,
				WebhookJobs:  redpanda.DefaultTopicsConfig().WebhookJobs,
				AuditStreams: redpanda.DefaultTopicsConfig().AuditStreams,
				Partitions:   24,
				RetentionHrs: redpanda.DefaultRetentionHrs,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			topics := tc.redpanda.Topics()
			assert.Equal(t, tc.expectedResult, topics)
		})
	}
}

func TestWebhookConfigRetryPolicy(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		webhook          config.WebhookConfig
		expectedSteps    []time.Duration
		expectedAttempts int
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "empty steps rejected at wiring time",
			webhook:          config.WebhookConfig{},
			expectedSteps:    nil,
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: retry steps are required (see config.yaml)"),
		},
		{
			name: "custom steps map and bound attempts",
			webhook: config.WebhookConfig{
				RetryStepsSec: []int{1, 5, 15},
			},
			expectedSteps:    []time.Duration{time.Second, 5 * time.Second, 15 * time.Second},
			expectedAttempts: 4,
			expectedError:    nil,
		},
		{
			name: "zero step rejected at wiring time",
			webhook: config.WebhookConfig{
				RetryStepsSec: []int{0},
			},
			expectedSteps:    nil,
			expectedAttempts: 0,
			expectedError:    errors.New("webhook: retry steps must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policy, err := tc.webhook.RetryPolicy()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
			require.NotNil(t, policy)
			assert.Equal(t, tc.expectedAttempts, policy.MaxAttempts())

			if tc.expectedSteps != nil {
				assert.Equal(t, tc.expectedSteps, policy.Schedule())
			} else {
				require.Len(t, policy.Schedule(), 7)
			}
		})
	}
}
