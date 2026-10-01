package apikey

import "errors"

// Sentinel errors for API keys.
var (
	ErrConfigRequired = errors.New("apikey: config is required")
	ErrClockRequired  = errors.New("apikey: clock is required")
	ErrNotFound       = errors.New("apikey: key not found")
	ErrExpired        = errors.New("apikey: key expired")
	ErrRevoked        = errors.New("apikey: key revoked")
	ErrForbiddenScope = errors.New("apikey: scope forbidden")
	ErrInvalidFormat  = errors.New("apikey: invalid key format")
	ErrNotInitialized = errors.New("apikey: not initialized")
)
