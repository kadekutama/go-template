package repository

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrDuplicateCreateMustFail = errors.New("duplicate Create must fail")
	ErrCrossTenantReadMustFail = errors.New("cross-tenant read must fail")
	ErrVersionConflict         = errors.New("version conflict")
	ErrAccountNotFound         = errors.New("account not found")
)
