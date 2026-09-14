package valueobject

import (
	"fmt"
)

// MatchDecision records whether a comparison group matched or broke.
type MatchDecision string

// Match decisions.
const (
	MatchMatched MatchDecision = "MATCHED"
	MatchBreak   MatchDecision = "BREAK"
)

// ParseMatchDecision validates a match decision.
func ParseMatchDecision(s string) (MatchDecision, error) {
	switch MatchDecision(s) {
	case MatchMatched, MatchBreak:
		return MatchDecision(s), nil
	default:
		return "", fmt.Errorf("reconciliation: invalid match decision %q", s)
	}
}
