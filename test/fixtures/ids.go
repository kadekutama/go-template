package fixtures

// Stable fixture identifiers shared across builders (single source of truth).
// Identity values are canonical UUID literals so fixtures insert explicitly
// under native UUID columns (ADR-019); constant names are preserved so
// consumers do not churn.
const (
	// DefaultTenantID is the stable default tenant.
	DefaultTenantID = "10000000-0000-4000-8000-000000000001"
	// DefaultTenantAlias is the stable default tenant slug.
	DefaultTenantAlias = "test-tenant-01"
	// DefaultLedgerID is the stable default ledger.
	DefaultLedgerID = "20000000-0000-4000-8000-000000000001"
	// DefaultLedgerAlias is the stable default ledger slug.
	DefaultLedgerAlias = "test-ledger-01"
	// AssetUSD, AssetEUR, AssetIDR form the default chart assets.
	AssetUSD = "USD"
	AssetEUR = "EUR"
	AssetIDR = "IDR"
	// SideDebit and SideCredit are the journal sides.
	SideDebit  = "DEBIT"
	SideCredit = "CREDIT"
	// PostingTransfer01 is the stable transfer posting referenced by workflows.
	PostingTransfer01 = "40000000-0000-4000-8000-000000000002"
	// PostingFunding01 is the stable funding posting.
	PostingFunding01 = "40000000-0000-4000-8000-000000000001"
	// PostingFee01 is the stable fee posting.
	PostingFee01 = "40000000-0000-4000-8000-000000000003"
	// PostingRefund01 is the stable refund posting.
	PostingRefund01 = "40000000-0000-4000-8000-000000000004"
	// Stable chart account IDs (single source of truth for builders).
	AcctOperatingUSD = "30000000-0000-4000-8000-000000000001"
	AcctOperatingEUR = "30000000-0000-4000-8000-000000000002"
	AcctOperatingIDR = "30000000-0000-4000-8000-000000000003"
	AcctFeeUSD       = "30000000-0000-4000-8000-000000000004"
	AcctFeeEUR       = "30000000-0000-4000-8000-000000000005"
	AcctFeeIDR       = "30000000-0000-4000-8000-000000000006"
	AcctSuspenseUSD  = "30000000-0000-4000-8000-000000000007"
	AcctSuspenseEUR  = "30000000-0000-4000-8000-000000000008"
	AcctSuspenseIDR  = "30000000-0000-4000-8000-000000000009"
)
