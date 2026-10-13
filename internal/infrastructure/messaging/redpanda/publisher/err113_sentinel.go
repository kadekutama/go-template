package publisher

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrPublisherTopicIsRequired                      = errors.New("publisher: topic is required")
	ErrPublisherBrokerIsRequired                     = errors.New("publisher: broker is required")
	ErrPublisherNotInitialized                       = errors.New("publisher: not initialized")
	ErrPublisherTenantIsRequired                     = errors.New("publisher: tenant is required")
	ErrPublisherEventTypeIsRequired                  = errors.New("publisher: event type is required")
	ErrPublisherAggregateIDIsRequired                = errors.New("publisher: aggregate id is required")
	ErrPublisherPayloadIsRequired                    = errors.New("publisher: payload is required")
	ErrBrokerDown                                    = errors.New("broker down")
	ErrPublisherPublishTransferCompletedV1BrokerDown = errors.New("publisher: publish transfer.completed.v1: broker down")
)
