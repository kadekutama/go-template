package fx

import "errors"

// Test-only static sentinels; err113 requires wrapped/static errors, and these
// names carry the scenario intent.
var (
	errTransportFailed = errors.New("fx test: transport failed")
	errCacheReadFailed = errors.New("fx test: cache read failed")
)
