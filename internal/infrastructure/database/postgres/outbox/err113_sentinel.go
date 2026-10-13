package outbox

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrOutboxWriterNeedsDB              = errors.New("outbox: writer needs DB")
	ErrOutboxTransactionIsRequired      = errors.New("outbox: transaction is required")
	ErrOutboxPollerNeedsAPublisher      = errors.New("outbox: poller needs a publisher")
	ErrOutboxMaxBatchMustBePositive     = errors.New("outbox: max batch must be positive")
	ErrOutboxBaseBackoffMustBePositive  = errors.New("outbox: base backoff must be positive")
	ErrOutboxClaimTimeoutMustBePositive = errors.New("outbox: claim timeout must be positive")
	ErrOutboxDBIsRequired               = errors.New("outbox: DB is required")
	ErrBrokerDown                       = errors.New("broker down")
)
