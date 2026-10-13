package coalesce

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrProviderDown = errors.New("provider down")
)
