package webhook

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrWebhookInvalidEndpoint1ViolationSIDRuleRequiredOnValue    = errors.New("webhook: invalid endpoint (1 violation(s)): ID: rule \"required\" on value : validation violation")
	ErrWebhookInvalidEndpoint1ViolationSTenantRuleRequiredOnValu = errors.New("webhook: invalid endpoint (1 violation(s)): Tenant: rule \"required\" on value : validation violation")
	ErrWebhookInvalidEndpoint1ViolationSURLRuleHTTPURLOnValueFtp = errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"http_url\" on value ftp://x: validation violation")
	ErrWebhookInvalidEndpoint1ViolationSURLRuleRequiredOnValue   = errors.New("webhook: invalid endpoint (1 violation(s)): URL: rule \"required\" on value : validation violation")
	ErrWebhookInvalidEndpoint1ViolationSEventsRuleRequiredOnValu = errors.New("webhook: invalid endpoint (1 violation(s)): Events: rule \"required\" on value []: validation violation")
	ErrWebhookInvalidEndpoint1ViolationSEventsRuleMinOnValue     = errors.New("webhook: invalid endpoint (1 violation(s)): Events: rule \"min\" on value []: validation violation")
	ErrWebhookInvalidEndpoint1ViolationSEvents0RuleRequiredOnVal = errors.New("webhook: invalid endpoint (1 violation(s)): Events[0]: rule \"required\" on value : validation violation")
	ErrWebhookInvalidEndpoint1ViolationSSecretRuleRequiredOnValu = errors.New("webhook: invalid endpoint (1 violation(s)): Secret: rule \"required\" on value : validation violation")
	ErrWebhookRegistryIsNotInitialized                           = errors.New("webhook: registry is not initialized")
	ErrWebhookAtLeastOneRetryStepIsRequired                      = errors.New("webhook: at least one retry step is required")
	ErrWebhookRetryStepsMustBePositive                           = errors.New("webhook: retry steps must be positive")
	ErrUnderlyingNetworkTimeout                                  = errors.New("underlying network timeout")
	ErrWebhookRecordFailed                                       = errors.New("webhook: record failed")
	ErrWebhookInvalidSignerParams1ViolationSSecretsRuleMinOnValu = errors.New("webhook: invalid signer params (1 violation(s)): Secrets: rule \"min\" on value []: validation violation")
	ErrWebhookAtLeastOneSecretIsRequired                         = errors.New("webhook: at least one secret is required")
	ErrWebhookInvalidSignerParams1ViolationSToleranceRuleRequire = errors.New("webhook: invalid signer params (1 violation(s)): Tolerance: rule \"required\" on value 0s: validation violation")
	ErrWebhookInvalidSignerParams1ViolationSToleranceRuleGtOnVal = errors.New("webhook: invalid signer params (1 violation(s)): Tolerance: rule \"gt\" on value -1s: validation violation")
	ErrWebhookSignerIsNotInitialized                             = errors.New("webhook: signer is not initialized")
	ErrWebhookSignatureMismatch                                  = errors.New("webhook: signature mismatch")
	ErrWebhookTimestampOutsideTolerance                          = errors.New("webhook: timestamp outside tolerance")
	ErrWebhookSignatureValuesMustStartWithV1                     = errors.New("webhook: signature values must start with 'v1='")
	ErrWebhookSignatureIsNotHex                                  = errors.New("webhook: signature is not hex")
	ErrWebhookSignatureIsRequired                                = errors.New("webhook: signature is required")
	ErrWebhookPayloadIsRequired                                  = errors.New("webhook: payload is required")
	ErrWebhookToleranceMustBePositive                            = errors.New("webhook: tolerance must be positive")
)
