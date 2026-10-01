package oauth2

import "errors"

// Sentinel errors for OAuth login. Provider details never leak beyond codes.
var (
	ErrConfigRequired  = errors.New("oauth2: config is required")
	ErrClockRequired   = errors.New("oauth2: clock is required")
	ErrUnknownProvider = errors.New("oauth2: unknown provider")
	ErrStateInvalid    = errors.New("oauth2: invalid state")
	ErrStateExpired    = errors.New("oauth2: expired state")
	ErrCodeReuse       = errors.New("oauth2: authorization code already used")
	ErrProviderFailure = errors.New("oauth2: provider failure")
	ErrNotInitialized  = errors.New("oauth2: not initialized")
)
