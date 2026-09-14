// Package pagination offers offset and opaque-cursor paging helpers (E01-T06).
//
// Handlers return ordinary (value, error) pairs; there is no Result monad.
// See cursor.go for the opaque token contract.
package pagination

import (
	"strconv"
)

// Defaults bound handler paging.
const (
	DefaultLimit = 50
	MaxLimit     = 200
)

// PageRequest is an offset page: Limit in (0, MaxLimit], Offset >= 0.
type PageRequest struct {
	Limit  int
	Offset int
}

// Normalize applies defaults and bounds; it never errors.
func (r PageRequest) Normalize() PageRequest {
	if r.Limit <= 0 {
		r.Limit = DefaultLimit
	}
	if r.Limit > MaxLimit {
		r.Limit = MaxLimit
	}
	if r.Offset < 0 {
		r.Offset = 0
	}
	return r
}

// PageResult pairs items with the total count for offset paging.
type PageResult[T any] struct {
	Items  []T
	Total  int64
	Limit  int
	Offset int
}

// ParseLimitOffset converts raw query values (e.g. ?limit= &offset=) into a
// normalized PageRequest; garbage falls back to defaults, never errors.
func ParseLimitOffset(limitRaw, offsetRaw string) PageRequest {
	limit, err := strconv.Atoi(limitRaw)
	if limitRaw == "" || err != nil {
		limit = DefaultLimit
	}
	offset, err := strconv.Atoi(offsetRaw)
	if offsetRaw == "" || err != nil || offset < 0 {
		offset = 0
	}
	return PageRequest{Limit: limit, Offset: offset}.Normalize()
}
