package service

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrFxPairBaseAndQuoteMustDiffer    = errors.New("fx: pair base and quote must differ")
	ErrFxPairRequiresBaseAndQuoteCodes = errors.New("fx: pair requires base and quote codes")
	ErrBoom                            = errors.New("boom")
)
