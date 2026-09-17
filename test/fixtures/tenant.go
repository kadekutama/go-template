// Package fixtures provides deterministic, stable-ID builders for
// integration suites (E07-T09). Builders are pure: no I/O, no randomness,
// byte-identical output for identical input.
package fixtures

// TenantFixture is a deterministic tenant seed.
type TenantFixture struct {
	TenantID string
	LedgerID string
	Name     string
	Region   string
}

// Tenant returns the stable default tenant fixture.
func Tenant(id string) TenantFixture {
	if id == "" {
		id = DefaultTenantID
	}

	return TenantFixture{
		TenantID: id,
		LedgerID: DefaultLedgerID,
		Name:     "Test Tenant 01",
		Region:   "local",
	}
}
