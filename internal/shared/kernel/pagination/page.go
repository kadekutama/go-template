// Package pagination offers offset and opaque-cursor paging helpers (E01-T06).
//
// Handlers return ordinary (value, error) pairs; there is no Result monad.
// See cursor.go for the opaque token contract.
package pagination

import (
	"errors"
	"strconv"
)

// Bounds for handler paging.
const (
	MaxLimit = 200
)

// PageRequest is an offset page: Limit in [1, MaxLimit], Offset >= 0.
type PageRequest struct {
	Limit  int
	Offset int
}

// Validate ensures Limit and Offset are within valid bounds.
func (r PageRequest) Validate() error {
	if r.Limit <= 0 || r.Limit > MaxLimit {
		return errors.New("pagination: limit must be between 1 and 200")
	}
	if r.Offset < 0 {
		return errors.New("pagination: offset must be non-negative")
	}
	return nil
}

// PageResult pairs items with the total count for offset paging.
type PageResult[T any] struct {
	Items  []T
	Total  int64
	Limit  int
	Offset int
}

// ParseLimitOffset converts raw query values into a validated PageRequest.
// It returns an error if limit or offset are missing, non-numeric, or out of bounds.
func ParseLimitOffset(limitRaw, offsetRaw string) (PageRequest, error) {
	if limitRaw == "" {
		return PageRequest{}, errors.New("pagination: limit is required")
	}
	limit, err := strconv.Atoi(limitRaw)
	if err != nil {
		return PageRequest{}, errors.New("pagination: invalid limit integer")
	}

	if offsetRaw == "" {
		return PageRequest{}, errors.New("pagination: offset is required")
	}
	offset, err := strconv.Atoi(offsetRaw)
	if err != nil {
		return PageRequest{}, errors.New("pagination: invalid offset integer")
	}

	req := PageRequest{Limit: limit, Offset: offset}
	if err := req.Validate(); err != nil {
		return PageRequest{}, err
	}
	return req, nil
}
