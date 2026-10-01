package openbao

import "errors"

// Sentinel errors for secrets. Paths appear in errors; values never do.
// Not-found (the named secret does not exist) is distinct from unavailable
// (transport, non-404 status, or decode failure): both fail closed, but
// callers can tell misconfiguration from an outage without parsing messages.
var (
	ErrConfigRequired    = errors.New("openbao: config is required")
	ErrClockRequired     = errors.New("openbao: clock is required")
	ErrSecretNotFound    = errors.New("openbao: secret not found")
	ErrSecretUnavailable = errors.New("openbao: secret unavailable")
	ErrLeaseFailed       = errors.New("openbao: lease failed")
	ErrNotInitialized    = errors.New("openbao: not initialized")
)
