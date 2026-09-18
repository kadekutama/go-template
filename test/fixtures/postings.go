package fixtures

// PostingLine is one deterministic journal line in minor units.
type PostingLine struct {
	AccountID   string
	Side        string
	AmountMinor int64
	AssetCode   string
}

// PostingFixture is a deterministic balanced posting seed.
type PostingFixture struct {
	PostingID   string
	TenantID    string
	LedgerID    string
	Operation   string
	Description string
	Lines       []PostingLine
}

// Postings returns stable balanced postings covering the core money-flow
// patterns: funding, transfer, fee, and refund.
func Postings(tenantID string, ledgerID string) []PostingFixture {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}

	if ledgerID == "" {
		ledgerID = DefaultLedgerID
	}

	return []PostingFixture{
		{
			PostingID:   PostingFunding01,
			TenantID:    tenantID,
			LedgerID:    ledgerID,
			Operation:   "FUNDING",
			Description: "initial funding",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 100000, AssetCode: AssetUSD},
				{AccountID: AcctSuspenseUSD, Side: SideCredit, AmountMinor: 100000, AssetCode: AssetUSD},
			},
		},
		{
			PostingID:   PostingTransfer01,
			TenantID:    tenantID,
			LedgerID:    ledgerID,
			Operation:   "TRANSFER",
			Description: "test transfer",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideCredit, AmountMinor: 25000, AssetCode: AssetUSD},
				{AccountID: AcctOperatingEUR, Side: SideDebit, AmountMinor: 25000, AssetCode: AssetUSD},
			},
		},
		{
			PostingID:   PostingFee01,
			TenantID:    tenantID,
			LedgerID:    ledgerID,
			Operation:   "FEE",
			Description: "test fee",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 290, AssetCode: AssetUSD},
				{AccountID: AcctFeeUSD, Side: SideCredit, AmountMinor: 290, AssetCode: AssetUSD},
			},
		},
		{
			PostingID:   PostingRefund01,
			TenantID:    tenantID,
			LedgerID:    ledgerID,
			Operation:   "REFUND",
			Description: "test refund",
			Lines: []PostingLine{
				{AccountID: AcctOperatingUSD, Side: SideDebit, AmountMinor: 5000, AssetCode: AssetUSD},
				{AccountID: AcctSuspenseUSD, Side: SideCredit, AmountMinor: 5000, AssetCode: AssetUSD},
			},
		},
	}
}
