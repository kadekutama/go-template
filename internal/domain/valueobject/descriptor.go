package valueobject

import (
	"fmt"
	"strings"
)

// ValidateDescriptor enforces network descriptor rules: ≤22 chars and an
// explicit charset (letters, digits, space, and .,-*'& allowed; reject
// control characters and other symbols).
func ValidateDescriptor(s, network string) error {
	if s == "" {
		return fmt.Errorf("descriptor: value is required")
	}
	if len([]rune(s)) > 22 {
		return fmt.Errorf("descriptor: INVALID_DESCRIPTOR exceeds 22 characters")
	}
	for _, r := range s {
		if err := checkDescriptorRune(r); err != nil {
			return err
		}
	}
	if network == "" {
		return fmt.Errorf("descriptor: network is required")
	}
	return nil
}

func checkDescriptorRune(r rune) error {
	if r < 0x20 || r == 0x7f {
		return fmt.Errorf("descriptor: INVALID_DESCRIPTOR control character")
	}
	if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
		return nil
	}
	if strings.ContainsRune(" .,-*'&", r) {
		return nil
	}
	return fmt.Errorf("descriptor: INVALID_DESCRIPTOR illegal character %q", r)
}
