package testcontainers

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrCitusHandleIsNotInitialized = errors.New("citus handle is not initialized")
)
