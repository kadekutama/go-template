package aggregate

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrEventLedgerIDIsRequired   = errors.New("event: ledger_id is required")
	ErrEventEventIDIsRequired    = errors.New("event: event_id is required")
	ErrEventOccurredAtIsRequired = errors.New("event: occurred_at is required")
)
