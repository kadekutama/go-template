package audit

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrWormUnavailable = errors.New("worm unavailable")
	ErrWormError       = errors.New("worm error")
)
