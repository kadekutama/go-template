package fixtures

// Stable fixture identifiers shared across builders (single source of truth).
const (
	// DefaultTenantID is the stable default tenant.
	DefaultTenantID = "tnt-test-01"
	// DefaultLedgerID is the stable default ledger.
	DefaultLedgerID = "ldg-test-01"
	// AssetUSD, AssetEUR, AssetIDR form the default chart assets.
	AssetUSD = "USD"
	AssetEUR = "EUR"
	AssetIDR = "IDR"
	// SideDebit and SideCredit are the journal sides.
	SideDebit  = "DEBIT"
	SideCredit = "CREDIT"
	// PostingTransfer01 is the stable transfer posting referenced by workflows.
	PostingTransfer01 = "pst-test-transfer-01"
	// Stable chart account IDs (single source of truth for builders).
	AcctOperatingUSD = "acct-test-operating-USD"
	AcctOperatingEUR = "acct-test-operating-EUR"
	AcctSuspenseUSD  = "acct-test-suspense-USD"
	AcctFeeUSD       = "acct-test-fee-USD"
)
