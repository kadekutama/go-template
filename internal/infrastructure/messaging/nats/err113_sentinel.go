package nats

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrNatsConsumerGroupIsRequired                               = errors.New("nats: consumer group is required")
	ErrNatsInvalidParams1ViolationSURLRuleRequiredOnValue        = errors.New("nats: invalid params (1 violation(s)): URL: rule \"required\" on value : validation violation")
	ErrNatsURLIsRequired                                         = errors.New("nats: url is required")
	ErrNatsInvalidParams1ViolationSConnectTimeoutRuleRequiredOnV = errors.New("nats: invalid params (1 violation(s)): ConnectTimeout: rule \"required\" on value 0s: validation violation")
	ErrNatsInvalidParams1ViolationSConnectTimeoutRuleGtOnValue1s = errors.New("nats: invalid params (1 violation(s)): ConnectTimeout: rule \"gt\" on value -1s: validation violation")
	ErrNatsInvalidParams1ViolationSRequestTimeoutRuleRequiredOnV = errors.New("nats: invalid params (1 violation(s)): RequestTimeout: rule \"required\" on value 0s: validation violation")
	ErrNatsInvalidParams1ViolationSRequestTimeoutRuleGtOnValue1s = errors.New("nats: invalid params (1 violation(s)): RequestTimeout: rule \"gt\" on value -1s: validation violation")
	ErrNatsSeedIsNotAUserNkey                                    = errors.New("nats: seed is not a user nkey")
	ErrNatsConnectNatsNoServersAvailableForConnection            = errors.New("nats: connect: nats: no servers available for connection")
	ErrNatsPublisherIsNotInitialized                             = errors.New("nats: publisher is not initialized")
	ErrNatsPayloadIsRequired                                     = errors.New("nats: payload is required")
	ErrNatsSubjectIsRequired                                     = errors.New("nats: subject is required")
)
