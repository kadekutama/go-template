package posting

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrPostgresUnitOfWorkNeedsDB                = errors.New("postgres: unit of work needs DB")
	ErrPostgresUnitOfWorkCallbackIsRequired     = errors.New("postgres: unit of work callback is required")
	ErrPostgresEntryTotalOverflow               = errors.New("postgres: entry total overflow")
	ErrPostgresHoldTotalOverflow                = errors.New("postgres: hold total overflow")
	ErrPostgresCompleteIdempotencyNoLeaseForKey = errors.New("postgres: complete idempotency: no lease for key")
)
