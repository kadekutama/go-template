package pagination

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrPaginationNegativeOffset            = errors.New("pagination: negative offset")
	ErrPaginationMalformedCursor           = errors.New("pagination: malformed cursor")
	ErrPaginationCursorChecksumMismatch    = errors.New("pagination: cursor checksum mismatch")
	ErrPaginationCursorOffsetOverflow      = errors.New("pagination: cursor offset overflow")
	ErrPaginationLimitMustBeBetween1And200 = errors.New("pagination: limit must be between 1 and 200")
	ErrPaginationOffsetMustBeNonNegative   = errors.New("pagination: offset must be non-negative")
	ErrPaginationInvalidLimitInteger       = errors.New("pagination: invalid limit integer")
	ErrPaginationLimitIsRequired           = errors.New("pagination: limit is required")
	ErrPaginationOffsetIsRequired          = errors.New("pagination: offset is required")
	ErrPaginationInvalidOffsetInteger      = errors.New("pagination: invalid offset integer")
)
