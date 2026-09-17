package fixtures

// StatementFixture is a deterministic bank-statement seed for parser tests.
type StatementFixture struct {
	StatementID string
	Format      string
	Raw         string
}

// Statements returns stable statement seeds across supported formats.
func Statements() []StatementFixture {
	return []StatementFixture{
		{
			StatementID: "stmt-test-csv-01",
			Format:      "CSV",
			Raw:         "date,amount,currency,reference\n2026-09-01,250.00,USD,pst-test-transfer-01\n",
		},
		{
			StatementID: "stmt-test-mt940-01",
			Format:      "MT940",
			Raw:         ":20:STMT-01\n:25:ACCT-01\n:60F:C260901USD100000,00\n:61:2609010901DR250,00NTRFpst-test-transfer-01\n",
		},
	}
}
