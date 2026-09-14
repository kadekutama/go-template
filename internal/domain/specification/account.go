package specification

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TenantPair is a same-scope candidate for two tenant-scoped values.
type TenantPair struct {
	A valueobject.TenantID
	B valueobject.TenantID
}

type accountActive struct{}

// AccountActive passes only for ACTIVE accounts. Frozen and closed accounts
// fail with distinct stable codes; any other status fails ACCOUNT_INACTIVE.
func AccountActive() Specification[entity.AccountData] { return accountActive{} }

// Evaluate implements Specification.
func (accountActive) Evaluate(_ context.Context, a entity.AccountData) SpecResult {
	switch a.Status {
	case valueobject.StatusActive:
		return SpecResult{}
	case valueobject.StatusFrozen:
		return SpecResult{Violations: []Violation{{Code: "ACCOUNT_FROZEN", Message: "account is frozen"}}}
	case valueobject.StatusClosed:
		return SpecResult{Violations: []Violation{{Code: "ACCOUNT_CLOSED", Message: "account is closed"}}}
	default:
		return SpecResult{Violations: []Violation{{Code: "ACCOUNT_INACTIVE", Message: "account is not active"}}}
	}
}

// SameTenant passes when both tenant scopes are equal and non-empty.
func SameTenant() Specification[TenantPair] {
	return NewFuncSpec[TenantPair]("TENANT_MISMATCH", "tenant scopes differ",
		func(_ context.Context, p TenantPair) bool {
			return p.A != "" && p.A == p.B
		})
}
