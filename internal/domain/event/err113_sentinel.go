package event

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrEventAmountMinorMustBePositive                  = errors.New("event: amount_minor must be positive")
	ErrEventTenantIDIsRequired                         = errors.New("event: tenant_id is required")
	ErrEventEventIDIsRequired                          = errors.New("event: event_id is required")
	ErrEventOccurredAtIsRequired                       = errors.New("event: occurred_at is required")
	ErrEventReasonIsRequired                           = errors.New("event: reason is required")
	ErrEventCapturedMinorMustBePositive                = errors.New("event: captured_minor must be positive")
	ErrEventOutcomeMustBeWONOrLOST                     = errors.New("event: outcome must be WON or LOST")
	ErrEventItemCountMustBePositive                    = errors.New("event: item_count must be positive")
	ErrEventRefundAmountMinorMustBePositive            = errors.New("event: refund_amount_minor must be positive")
	ErrEventRateIsRequired                             = errors.New("event: rate is required")
	ErrEventPostedTransactionRequiresAtLeastTwoEntries = errors.New("event: posted transaction requires at least two entries")
	ErrEventAggregateIDIsRequired                      = errors.New("event: aggregate_id is required")
	ErrEventAggregateTypeIsRequired                    = errors.New("event: aggregate_type is required")
	ErrEventEventTypeIsRequired                        = errors.New("event: event_type is required")
	ErrEventMetadataTenantIDIsRequired                 = errors.New("event: metadata tenant_id is required")
	ErrEventAggregateVersionMustNotBeNegative          = errors.New("event: aggregate_version must not be negative")
	ErrEventSequenceMustNotBeNegative                  = errors.New("event: sequence must not be negative")
	ErrEventNameIsRequired                             = errors.New("event: name is required")
)
