package egress

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrValkeyDown = errors.New("valkey down")
)
