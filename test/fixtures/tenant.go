// Package fixtures provides deterministic, stable-ID builders for
// integration suites (E07-T09). Builders are pure: no I/O, no randomness,
// byte-identical output for identical input.
package fixtures

// TenantFixture is a deterministic tenant seed.
type TenantFixture struct {
	TenantID string
	LedgerID string
	Name     string
	Alias    string
	Region   string
}

// Tenant returns a deterministic tenant fixture. Empty id and alias resolve
// to the stable defaults; explicit values pass through untouched so every
// caller owns alias uniqueness (aliases are globally UNIQUE).
func Tenant(id string, alias string) TenantFixture {
	if id == "" {
		id = DefaultTenantID
	}

	if alias == "" {
		alias = DefaultTenantAlias
	}

	return TenantFixture{
		TenantID: id,
		LedgerID: DefaultLedgerID,
		Name:     "Test Tenant 01",
		Alias:    alias,
		Region:   "local",
	}
}
