package fixtures

// AccountFixture is a deterministic account seed.
type AccountFixture struct {
	AccountID string
	TenantID  string
	LedgerID  string
	AssetCode string
	Kind      string
}

// Accounts returns the stable default chart: operating, fee, and suspense
// accounts across USD, EUR, and IDR.
func Accounts(tenantID string, ledgerID string) []AccountFixture {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}

	if ledgerID == "" {
		ledgerID = DefaultLedgerID
	}

	kinds := []struct {
		suffix string
		kind   string
	}{
		{suffix: "operating", kind: "OPERATING"},
		{suffix: "fee", kind: "FEE"},
		{suffix: "suspense", kind: "SUSPENSE"},
	}
	assets := []string{AssetUSD, AssetEUR, AssetIDR}

	out := make([]AccountFixture, 0, len(kinds)*len(assets))
	for _, kind := range kinds {
		for _, asset := range assets {
			out = append(out, AccountFixture{
				AccountID: "acct-test-" + kind.suffix + "-" + asset,
				TenantID:  tenantID,
				LedgerID:  ledgerID,
				AssetCode: asset,
				Kind:      kind.kind,
			})
		}
	}

	return out
}
