package fixtures

// WorkflowFixture is a deterministic non-ledger workflow seed (transfers,
// payouts, disputes) referencing the stable ledger fixtures.
type WorkflowFixture struct {
	WorkflowID string
	TenantID   string
	Kind       string
	Status     string
	PostingID  string
}

// Workflows returns stable workflow seeds for integration suites.
func Workflows(tenantID string) []WorkflowFixture {
	if tenantID == "" {
		tenantID = DefaultTenantID
	}

	return []WorkflowFixture{
		{WorkflowID: "wfl-test-transfer-01", TenantID: tenantID, Kind: "TRANSFER", Status: "PENDING", PostingID: PostingTransfer01},
		{WorkflowID: "wfl-test-payout-01", TenantID: tenantID, Kind: "PAYOUT", Status: "PENDING", PostingID: ""},
		{WorkflowID: "wfl-test-dispute-01", TenantID: tenantID, Kind: "DISPUTE", Status: "OPEN", PostingID: PostingTransfer01},
	}
}
