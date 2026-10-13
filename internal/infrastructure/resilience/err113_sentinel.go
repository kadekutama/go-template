package resilience

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrDown      = errors.New("down")
	ErrFlaky     = errors.New("flaky")
	ErrTemporary = errors.New("temporary")
)
