package rls

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrRlsTenantIsRequired                       = errors.New("rls: tenant is required")
	ErrRlsTenantMissingFromContext               = errors.New("rls: tenant missing from context")
	ErrRlsContextTenantAndArgumentTenantDisagree = errors.New("rls: context tenant and argument tenant disagree")
	ErrRlsDBIsRequired                           = errors.New("rls: DB is required")
)
