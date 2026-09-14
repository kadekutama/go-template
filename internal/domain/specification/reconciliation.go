package specification

import (
	"math"
	"slices"
	"time"
)

// AmountWithinTolerance reports whether |ledgerSum-externalSum| fits tolerance.
func AmountWithinTolerance(ledgerSum, externalSum, toleranceMinor int64) bool {
	if toleranceMinor < 0 {
		return false
	}
	diff := ledgerSum - externalSum
	if (ledgerSum > 0 && externalSum < 0 && diff < 0) || (ledgerSum < 0 && externalSum > 0 && diff > 0) {
		return false
	}
	if diff < 0 {
		if diff == math.MinInt64 {
			return false
		}
		diff = -diff
	}
	return diff <= toleranceMinor
}

// TimingWithinWindow reports whether skew fits the timing window.
func TimingWithinWindow(skew, window time.Duration) bool {
	if window < 0 {
		return false
	}
	if skew < 0 {
		if skew == time.Duration(math.MinInt64) {
			return false
		}
		skew = -skew
	}
	return skew <= window
}

// CanAutoResolveTiming reports whether a timing-kind break may auto-close.
// Amount mismatches and duplicates never auto-resolve.
func CanAutoResolveTiming(breakType string, allowed []string, skew, maxSkew time.Duration) bool {
	if breakType == "AMOUNT_MISMATCH" || breakType == "DUPLICATE" {
		return false
	}
	if !slices.Contains(allowed, breakType) {
		return false
	}
	return TimingWithinWindow(skew, maxSkew)
}
