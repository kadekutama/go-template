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

	kinds := []string{"OPERATING", "FEE", "SUSPENSE"}
	assets := []string{AssetUSD, AssetEUR, AssetIDR}

	// Deterministic account IDs keyed by kind/asset (single source of truth
	// for the chart; postings.go references the same constants).
	accountIDs := map[string]string{
		"OPERATING/USD": AcctOperatingUSD,
		"OPERATING/EUR": AcctOperatingEUR,
		"OPERATING/IDR": AcctOperatingIDR,
		"FEE/USD":       AcctFeeUSD,
		"FEE/EUR":       AcctFeeEUR,
		"FEE/IDR":       AcctFeeIDR,
		"SUSPENSE/USD":  AcctSuspenseUSD,
		"SUSPENSE/EUR":  AcctSuspenseEUR,
		"SUSPENSE/IDR":  AcctSuspenseIDR,
	}

	out := make([]AccountFixture, 0, len(kinds)*len(assets))
	for _, kind := range kinds {
		for _, asset := range assets {
			out = append(out, AccountFixture{
				AccountID: accountIDs[kind+"/"+asset],
				TenantID:  tenantID,
				LedgerID:  ledgerID,
				AssetCode: asset,
				Kind:      kind,
			})
		}
	}

	return out
}
