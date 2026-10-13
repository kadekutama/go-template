package validate

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrValidateViolation = errors.New("validation violation")
)
