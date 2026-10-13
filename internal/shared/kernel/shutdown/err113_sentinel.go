package shutdown

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrServeBoom                          = errors.New("serve boom")
	ErrDrainFailure                       = errors.New("drain failure")
	ErrShutdownServeFuncMustNotBeNil      = errors.New("shutdown: serve func must not be nil")
	ErrShutdownDrainTimeoutMustBePositive = errors.New("shutdown: drain timeout must be positive")
)
