package profiling

import "errors"

// Static error sentinels for err113 and errors.Is call sites.
var (
	ErrNilLogger         = errors.New("profiling: logger is required")
	ErrNotInitialized    = errors.New("profiling: profiler is not initialized")
	ErrEmptyTagKey       = errors.New("profiling: tag key must not be empty")
	ErrForbiddenTagKey   = errors.New("profiling: forbidden high-cardinality/PII tag key")
	ErrUnsupportedTagKey = errors.New("profiling: unsupported tag key (allowed: service, env, region, version)")
	ErrStartFailed       = errors.New("profiling: start profiler")
	ErrStopFailed        = errors.New("profiling: stop profiler")
)
