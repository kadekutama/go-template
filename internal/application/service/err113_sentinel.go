package service

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrDbConnectionFailure = errors.New("db connection failure")
	ErrCustomErrorMessage  = errors.New("custom error message")
)
