package rbac

import "errors"

// Sentinel errors for authorization.
var (
	ErrForbidden     = errors.New("rbac: forbidden")
	ErrPolicyInvalid = errors.New("rbac: invalid policy")
	ErrNotInit       = errors.New("rbac: not initialized")
)
