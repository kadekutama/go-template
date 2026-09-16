package command

const (
	// DefaultPageLimit is the fallback limit when none is specified or limit <= 0.
	DefaultPageLimit = 50
	// MaxPageLimit is the maximum items allowed in a single page.
	MaxPageLimit = 100
)

// clampPageLimit bounds list page sizes, defaulting non-positive limits.
// Shared by every list path so pagination behaves identically.
func clampPageLimit(limit int) int {
	if limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}
