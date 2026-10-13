package resilience

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrResilienceMaxAttemptsMustBePositive                       = errors.New("resilience: max attempts must be positive")
	ErrResilienceInitialBackoffMustBePositive                    = errors.New("resilience: initial backoff must be positive")
	ErrResilienceMaxBackoffMustBeGreaterThanOrEqualToInitialBack = errors.New("resilience: max backoff must be greater than or equal to initial backoff")
	ErrResilienceMultiplierMustBeAtLeast10                       = errors.New("resilience: multiplier must be at least 1.0")
	ErrX                                                         = errors.New("x")
	ErrUnderlyingIssue                                           = errors.New("underlying issue")
)
