package port

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrBoom          = errors.New("boom")
	ErrProcessorDown = errors.New("processor down")
)
