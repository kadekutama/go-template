package config_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/auth/apikey"
	"github.com/kadekutama/go-template/internal/infrastructure/cache/hybrid"
	"github.com/kadekutama/go-template/internal/infrastructure/config"
	"github.com/kadekutama/go-template/internal/infrastructure/coordination/etcd"
	"github.com/kadekutama/go-template/internal/infrastructure/httpclient"
	"github.com/kadekutama/go-template/internal/infrastructure/logging"
	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
	"github.com/kadekutama/go-template/internal/infrastructure/tracing"
)

func TestCacheTTLsConfigHybridTTLs(t *testing.T) {
	t.Parallel()

	validConfig := config.CacheTTLsConfig{
		BalanceSec:         60,
		BalanceCursorSec:   300,
		ConfigSec:          300,
		ConfigLongSec:      1800,
		FXSec:              3600,
		IdempotencyHintSec: 86400,
		RateLimitSec:       60,
		L1PopulateSec:      60,
	}

	expectedValid := hybrid.TTLSet{
		Balance:         60 * time.Second,
		BalanceCursor:   300 * time.Second,
		Config:          300 * time.Second,
		ConfigLong:      1800 * time.Second,
		FX:              3600 * time.Second,
		IdempotencyHint: 86400 * time.Second,
		RateLimit:       60 * time.Second,
		L1Populate:      60 * time.Second,
	}

	type testCase struct {
		name           string
		ttls           config.CacheTTLsConfig
		expectedResult hybrid.TTLSet
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid config maps through",
			ttls:           validConfig,
			expectedResult: expectedValid,
			expectedError:  nil,
		},
		{
			name:           "zero config rejected",
			ttls:           config.CacheTTLsConfig{},
			expectedResult: hybrid.TTLSet{},
			expectedError:  errors.New("cache: ttl balance must be positive"),
		},
		{
			name: "negative override rejected",
			ttls: func() config.CacheTTLsConfig {
				c := validConfig
				c.BalanceSec = -1
				return c
			}(),
			expectedResult: hybrid.TTLSet{},
			expectedError:  errors.New("cache: ttl balance must be positive"),
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

	validRedpanda := config.RedpandaConfig{
		Brokers:       []string{"127.0.0.1:9092"},
		LedgerEvents:  "ledger.events.v1",
		OutboxFacts:   "outbox.facts.v1",
		WebhookJobs:   "webhook.jobs.v1",
		AuditStreams:  "audit.streams.v1",
		Partitions:    12,
		RetentionHrs:  720,
		PartitionKeys: "tenant_id:account_id",
		DLQSuffix:     ".dlq",
	}

	expectedValid := redpanda.TopicsConfig{
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
		redpanda       config.RedpandaConfig
		expectedResult redpanda.TopicsConfig
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid config maps through",
			redpanda:       validRedpanda,
			expectedResult: expectedValid,
			expectedError:  nil,
		},
		{
			name:           "zero config rejected",
			redpanda:       config.RedpandaConfig{},
			expectedResult: redpanda.TopicsConfig{},
			expectedError:  errors.New("redpanda: ledger events topic is required"),
		},
		{
			name: "missing outbox facts rejected",
			redpanda: func() config.RedpandaConfig {
				c := validRedpanda
				c.OutboxFacts = ""
				return c
			}(),
			expectedResult: redpanda.TopicsConfig{},
			expectedError:  errors.New("redpanda: outbox facts topic is required"),
		},
		{
			name: "missing partition keys rejected",
			redpanda: func() config.RedpandaConfig {
				c := validRedpanda
				c.PartitionKeys = ""
				return c
			}(),
			expectedResult: redpanda.TopicsConfig{},
			expectedError:  errors.New("redpanda: partition keys is required"),
		},
		{
			name: "missing dlq suffix rejected",
			redpanda: func() config.RedpandaConfig {
				c := validRedpanda
				c.DLQSuffix = ""
				return c
			}(),
			expectedResult: redpanda.TopicsConfig{},
			expectedError:  errors.New("redpanda: dlq suffix is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			topics, err := tc.redpanda.Topics()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, topics)
				return
			}
			require.NoError(t, err)
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

func TestCoordinationConfigCoordination(t *testing.T) {
	t.Parallel()

	validCoord := config.CoordinationConfig{
		EtcdEndpoints:          []string{"http://127.0.0.1:2379"},
		EtcdDialTimeoutSec:     5,
		EtcdElectionTTLSeconds: 5,
		LeaderKeyPrefix:        "/finance/worker-leader",
	}

	expectedValid := etcd.Config{
		Endpoints:          []string{"http://127.0.0.1:2379"},
		DialTimeout:        5 * time.Second,
		ElectionTTLSeconds: 5,
		LeaderKeyPrefix:    "/finance/worker-leader",
	}

	type testCase struct {
		name           string
		coord          config.CoordinationConfig
		expectedResult etcd.Config
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid coordination maps through",
			coord:          validCoord,
			expectedResult: expectedValid,
			expectedError:  nil,
		},
		{
			name:           "zero coordination rejected",
			coord:          config.CoordinationConfig{},
			expectedResult: etcd.Config{},
			expectedError:  errors.New("etcd: at least one endpoint is required"),
		},
		{
			name: "missing prefix rejected",
			coord: func() config.CoordinationConfig {
				c := validCoord
				c.LeaderKeyPrefix = ""
				return c
			}(),
			expectedResult: etcd.Config{},
			expectedError:  errors.New("etcd: leader key prefix must not be blank"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := tc.coord.Coordination()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, cfg)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, cfg)
		})
	}
}

func TestHTTPClientConfig(t *testing.T) {
	t.Parallel()

	validConfig := config.HTTPClientConfig{
		TimeoutSec:          10,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeoutSec:  90,
	}

	expectedValid := httpclient.Params{
		Timeout:             10 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	type testCase struct {
		name           string
		cfg            config.HTTPClientConfig
		expectedResult httpclient.Params
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid http client maps through",
			cfg:            validConfig,
			expectedResult: expectedValid,
			expectedError:  nil,
		},
		{
			name:           "zero http client rejected",
			cfg:            config.HTTPClientConfig{},
			expectedResult: httpclient.Params{},
			expectedError:  errors.New("httpclient: all parameters must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := tc.cfg.HTTPClientParams()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, params)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, params)
		})
	}
}

func TestAPIKeyConfig(t *testing.T) {
	t.Parallel()

	validConfig := config.APIKeyConfig{
		Time:      3,
		MemoryKiB: 65536,
		Threads:   4,
		KeyLen:    32,
		SaltLen:   16,
	}

	expectedValid := apikey.HasherParams{
		Time:    3,
		Memory:  65536,
		Threads: 4,
		KeyLen:  32,
		SaltLen: 16,
	}

	type testCase struct {
		name           string
		cfg            config.APIKeyConfig
		expectedResult apikey.HasherParams
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid api key maps through",
			cfg:            validConfig,
			expectedResult: expectedValid,
			expectedError:  nil,
		},
		{
			name:           "zero api key rejected",
			cfg:            config.APIKeyConfig{},
			expectedResult: apikey.HasherParams{},
			expectedError:  errors.New("apikey: all parameters must be positive (threads 1-16)"),
		},
		{
			name: "threads above file ceiling rejected",
			cfg: func() config.APIKeyConfig {
				c := validConfig
				c.Threads = 17
				return c
			}(),
			expectedResult: apikey.HasherParams{},
			expectedError:  errors.New("apikey: all parameters must be positive (threads 1-16)"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := tc.cfg.HasherParams()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, params)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, params)
		})
	}
}

func TestObservabilityConfigLogging(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		cfg            config.ObservabilityConfig
		expectedResult logging.Config
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "valid log level maps through",
			cfg: config.ObservabilityConfig{
				LogLevel: "info",
			},
			expectedResult: logging.Config{Level: "info"},
			expectedError:  nil,
		},
		{
			name: "missing log level rejected",
			cfg: config.ObservabilityConfig{
				LogLevel: "",
			},
			expectedResult: logging.Config{},
			expectedError:  errors.New("logging: log_level is required"),
		},
		{
			name: "invalid log level rejected",
			cfg: config.ObservabilityConfig{
				LogLevel: "verbose",
			},
			expectedResult: logging.Config{},
			expectedError:  errors.New("logging: unknown log level \"verbose\""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.cfg.Logging()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tc.expectedResult, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestRedpandaTopicsDLQParity(t *testing.T) {
	t.Parallel()

	cfg, err := config.Load(filepath.Join("..", "..", "..", "config", "config.yaml"))
	require.NoError(t, err)

	topics, err := cfg.Redpanda.Topics()
	require.NoError(t, err)

	registry, err := redpanda.NewTopicRegistry(topics)
	require.NoError(t, err)

	type testCase struct {
		name          string
		topic         string
		expectedKnown bool
	}

	testCases := []testCase{
		{
			name:          "ledger events parity",
			topic:         registry.LedgerEventsTopic(),
			expectedKnown: true,
		},
		{
			name:          "outbox facts parity",
			topic:         registry.OutboxFactsTopic(),
			expectedKnown: true,
		},
		{
			name:          "webhook jobs parity",
			topic:         registry.WebhookJobsTopic(),
			expectedKnown: true,
		},
		{
			name:          "audit streams parity",
			topic:         registry.AuditStreamsTopic(),
			expectedKnown: true,
		},
		{
			name:          "unknown name maps to namespaced dlq",
			topic:         "custom.group",
			expectedKnown: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedKnown, registry.IsKnownTopic(tc.topic))

			dlq, err := registry.DLQFor(tc.topic)
			require.NoError(t, err)

			if !tc.expectedKnown {
				assert.Equal(t, tc.topic+topics.DLQSuffix, dlq)
				return
			}

			specs := registry.Registry()
			matched := false
			for _, spec := range specs {
				if spec.Name == tc.topic {
					assert.Equal(t, spec.DLQ, dlq)
					matched = true
				}
			}
			assert.True(t, matched)
		})
	}
}

func TestTenancyConfigAssetCap(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		maxAssets      int
		expectedResult int
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "configured cap maps through",
			maxAssets:      8,
			expectedResult: 8,
			expectedError:  nil,
		},
		{
			name:           "zero cap rejected",
			maxAssets:      0,
			expectedResult: 0,
			expectedError:  errors.New("tenancy: max_onboarding_assets must be positive (see config.yaml)"),
		},
		{
			name:           "negative cap rejected",
			maxAssets:      -2,
			expectedResult: 0,
			expectedError:  errors.New("tenancy: max_onboarding_assets must be positive (see config.yaml)"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := config.TenancyConfig{MaxOnboardingAssets: tc.maxAssets}.AssetCap()
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, 0, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}

func TestConfigTracing(t *testing.T) {
	t.Parallel()

	baseConfig := config.Config{
		App: config.AppConfig{Name: "ledger", Env: "staging"},
		Observability: config.ObservabilityConfig{
			LogLevel:       "info",
			OTLPEndpoint:   "https://tempo.staging.svc:4317",
			SampleRatio:    0.1,
			ExcludedRoutes: []string{"/healthz", "/readyz", "/livez"},
		},
	}

	type testCase struct {
		name           string
		cfg            config.Config
		serviceVersion string
		expectedResult tracing.Config
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid config maps through",
			cfg:            baseConfig,
			serviceVersion: "v1.2.3",
			expectedResult: tracing.Config{
				ServiceName:    "ledger",
				ServiceVersion: "v1.2.3",
				Env:            "staging",
				OTLPEndpoint:   "https://tempo.staging.svc:4317",
				SampleRatio:    0.1,
				ExcludedRoutes: []string{"/healthz", "/readyz", "/livez"},
			},
			expectedError: nil,
		},
		{
			name: "missing excluded routes rejected",
			cfg: func() config.Config {
				c := baseConfig
				c.Observability.ExcludedRoutes = nil
				return c
			}(),
			serviceVersion: "v1.2.3",
			expectedResult: tracing.Config{},
			expectedError:  errors.New("tracing: ExcludedRoutes is required"),
		},
		{
			name: "blank route rejected",
			cfg: func() config.Config {
				c := baseConfig
				c.Observability.ExcludedRoutes = []string{"/healthz", "   "}
				return c
			}(),
			serviceVersion: "v1.2.3",
			expectedResult: tracing.Config{},
			expectedError:  errors.New("tracing: excluded route must not be blank"),
		},
		{
			name: "zero sample ratio rejected",
			cfg: func() config.Config {
				c := baseConfig
				c.Observability.SampleRatio = 0
				return c
			}(),
			serviceVersion: "v1.2.3",
			expectedResult: tracing.Config{},
			expectedError:  errors.New("tracing: SampleRatio 0 out of (0,1]"),
		},
		{
			name:           "missing service version rejected",
			cfg:            baseConfig,
			serviceVersion: "",
			expectedResult: tracing.Config{},
			expectedError:  errors.New("tracing: ServiceVersion is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.cfg.Tracing(tc.serviceVersion)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Equal(t, tracing.Config{}, result)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expectedResult, result)
		})
	}
}
