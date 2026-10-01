package openbao

import (
	"strings"
)

// ResolveReference parses "{{ secret:path }}" into path. It reports false
// when ref is not a secret reference (plain config values pass through).
func ResolveReference(ref string) (path string, ok bool) {
	trimmed := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmed, "{{ secret:") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}

	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(trimmed, "{{ secret:"), "}}"))
	if inner == "" {
		return "", false
	}

	return inner, true
}
