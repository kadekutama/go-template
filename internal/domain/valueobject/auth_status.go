package valueobject

import (
	"fmt"
)

// AuthStatus is the authorization lifecycle state.
type AuthStatus string

// Authorization states.
const (
	AuthAuthorized        AuthStatus = "AUTHORIZED"
	AuthPartiallyCaptured AuthStatus = "PARTIALLY_CAPTURED"
	AuthCaptured          AuthStatus = "CAPTURED"
	AuthVoided            AuthStatus = "VOIDED"
	AuthExpired           AuthStatus = "EXPIRED"
	AuthRequiresAction    AuthStatus = "REQUIRES_ACTION"
)

// ParseAuthStatus validates an authorization state.
func ParseAuthStatus(s string) (AuthStatus, error) {
	switch AuthStatus(s) {
	case AuthAuthorized, AuthPartiallyCaptured, AuthCaptured, AuthVoided, AuthExpired, AuthRequiresAction:
		return AuthStatus(s), nil
	default:
		return "", fmt.Errorf("auth: invalid status %q", s)
	}
}

// CanTransitionAuth reports legal authorization moves: forward capture
// progress, challenge entry/resolution, or expiry from a live auth. Terminal
// states (CAPTURED, VOIDED, EXPIRED) have no outgoing moves.
func CanTransitionAuth(from, to AuthStatus) bool {
	switch from {
	case AuthAuthorized:
		return to == AuthPartiallyCaptured || to == AuthCaptured || to == AuthExpired || to == AuthVoided || to == AuthRequiresAction
	case AuthPartiallyCaptured:
		return to == AuthPartiallyCaptured || to == AuthCaptured || to == AuthExpired || to == AuthVoided
	case AuthRequiresAction:
		return to == AuthAuthorized || to == AuthVoided
	default:
		return false
	}
}
