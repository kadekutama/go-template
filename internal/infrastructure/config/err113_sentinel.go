package config

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrCacheTTLBalanceMustBePositive                         = errors.New("cache: ttl balance must be positive: cache: ttl must be positive")
	ErrRedpandaLedgerEventsTopicIsRequired                   = errors.New("redpanda: ledger events topic is required")
	ErrRedpandaOutboxFactsTopicIsRequired                    = errors.New("redpanda: outbox facts topic is required")
	ErrRedpandaPartitionKeysIsRequired                       = errors.New("redpanda: partition keys is required")
	ErrRedpandaDlqSuffixIsRequired                           = errors.New("redpanda: dlq suffix is required")
	ErrWebhookRetryStepsAreRequiredSeeConfigYaml             = errors.New("webhook: retry steps are required (see config.yaml)")
	ErrWebhookRetryStepsMustBePositive                       = errors.New("webhook: retry steps must be positive")
	ErrEtcdAtLeastOneEndpointIsRequired                      = errors.New("etcd: at least one endpoint is required")
	ErrEtcdLeaderKeyPrefixMustNotBeBlank                     = errors.New("etcd: leader key prefix must not be blank")
	ErrHttpclientAllParametersMustBePositive                 = errors.New("httpclient: all parameters must be positive")
	ErrApikeyAllParametersMustBePositiveThreads116           = errors.New("apikey: all parameters must be positive (threads 1-16)")
	ErrLoggingLogLevelIsRequired                             = errors.New("logging: log_level is required")
	ErrLoggingUnknownLogLevelVerbose                         = errors.New("logging: unknown log level \"verbose\": logging: unknown log level")
	ErrTenancyMaxOnboardingAssetsMustBePositiveSeeConfigYaml = errors.New("tenancy: max_onboarding_assets must be positive (see config.yaml)")
	ErrTracingExcludedRoutesIsRequired                       = errors.New("tracing: ExcludedRoutes is required")
	ErrTracingExcludedRouteMustNotBeBlank                    = errors.New("tracing: excluded route must not be blank")
	ErrTracingSampleRatio0OutOf01                            = errors.New("tracing: SampleRatio 0 out of (0,1]: tracing: SampleRatio out of (0,1]")
	ErrTracingServiceVersionIsRequired                       = errors.New("tracing: ServiceVersion is required")
	ErrWebhookToleranceSecMustBePositiveSeeConfigYaml        = errors.New("webhook: tolerance_sec must be positive (see config.yaml)")
	ErrApikeyParametersExceedUint32Range                     = errors.New("apikey: parameters exceed uint32 range")
	ErrInvalidConfigViolationS                               = errors.New("invalid config ( violation(s))")
)
