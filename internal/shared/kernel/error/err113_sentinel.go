package apperror

import "errors"

// Static error sentinels for err113.
var (
	ErrApperrorUnregisteredCode  = errors.New("apperror: unregistered code")
	ErrApperrorUnregisteredCode2 = errors.New("apperror: unregistered code")
)
