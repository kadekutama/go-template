package crypto

import "errors"

// Sentinel errors for envelope crypto. Plaintext never appears in errors.
var (
	ErrConfigRequired = errors.New("crypto: config is required")
	ErrAuthFailed     = errors.New("crypto: authentication failed")
	ErrUnknownKey     = errors.New("crypto: unknown key")
	ErrShredded       = errors.New("crypto: key shredded")
	ErrNotInitialized = errors.New("crypto: not initialized")
)
