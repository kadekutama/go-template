package local

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrOtterKeyIsRequired               = errors.New("otter: key is required")
	ErrOtterCacheIsNotInitialized       = errors.New("otter: cache is not initialized")
	ErrOtterTTLMustBePositive           = errors.New("otter: ttl must be positive")
	ErrOtterMaximumWeightMustBePositive = errors.New("otter: maximum weight must be positive")
	ErrOtterDefaultTTLMustBePositive    = errors.New("otter: default TTL must be positive")
)
