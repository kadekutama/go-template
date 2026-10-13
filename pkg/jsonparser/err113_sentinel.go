package jsonparser

import "errors"

// Static error sentinels for err113.
var (
	ErrJsonparserGetMissingKeyAtSegment        = errors.New("jsonparser: get: missing key at segment")
	ErrJsonparserGetBadIndexAtSegment          = errors.New("jsonparser: get: bad index at segment")
	ErrJsonparserGetCannotDescendIntoAtSegment = errors.New("jsonparser: get: cannot descend into at segment")
)
