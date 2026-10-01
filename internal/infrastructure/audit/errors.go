package audit

import "errors"

// Sentinel errors for audit. Entry contents never appear in errors.
var (
	ErrConfigRequired    = errors.New("audit: config is required")
	ErrClockRequired     = errors.New("audit: clock is required")
	ErrChainBroken       = errors.New("audit: chain broken")
	ErrCheckpointInvalid = errors.New("audit: checkpoint invalid")
	ErrSecretInEntry     = errors.New("audit: secrets must not enter audit entries")
	ErrNotInitialized    = errors.New("audit: not initialized")
)
