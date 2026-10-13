package hybrid

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrCacheTTLBalanceMustBePositive         = errors.New("cache: ttl balance must be positive: cache: ttl must be positive")
	ErrCacheTTLBalanceCursorMustBePositive   = errors.New("cache: ttl balance_cursor must be positive: cache: ttl must be positive")
	ErrCacheTTLConfigMustBePositive          = errors.New("cache: ttl config must be positive: cache: ttl must be positive")
	ErrCacheTTLConfigLongMustBePositive      = errors.New("cache: ttl config_long must be positive: cache: ttl must be positive")
	ErrCacheTTLFxMustBePositive              = errors.New("cache: ttl fx must be positive: cache: ttl must be positive")
	ErrCacheTTLIdempotencyHintMustBePositive = errors.New("cache: ttl idempotency_hint must be positive: cache: ttl must be positive")
	ErrCacheTTLRateLimitMustBePositive       = errors.New("cache: ttl rate_limit must be positive: cache: ttl must be positive")
	ErrCacheTTLL1PopulateMustBePositive      = errors.New("cache: ttl l1_populate must be positive: cache: ttl must be positive")
	ErrL2Down                                = errors.New("l2 down")
	ErrConnectionRefused                     = errors.New("connection refused")
	ErrPttlTimeout                           = errors.New("pttl timeout")
	ErrTTLMustBePositive                     = errors.New("ttl must be positive")
	ErrCacheTTLMustBePositive                = errors.New("cache: ttl must be positive")
)
