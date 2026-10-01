package command

import "github.com/kadekutama/go-template/internal/domain/entity"

const (
	// MinPageLimit is the minimum items allowed in a single page.
	MinPageLimit = 1
	// MaxPageLimit is the maximum items allowed in a single page.
	MaxPageLimit = 100
)

// validatePageLimit verifies that the caller explicitly supplied a valid page limit.
func validatePageLimit(limit int) error {
	if limit < MinPageLimit || limit > MaxPageLimit {
		return entity.NewError("INVALID_PAGE_LIMIT", "page limit must be between 1 and 100")
	}
	return nil
}
