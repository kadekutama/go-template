package service_test

import (
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD      = "USD"
	testEUR      = "EUR"
	testTenant1  = "t-1"
	testTenant2  = "t-2"
	testLedger1  = "l-1"
	testAccount1 = "a-1"
	testAccount2 = "a-2"
)

func periodAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: {ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "1000", Name: "usd", Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
		testAccount2: {ID: testAccount2, TenantID: testTenant1, LedgerID: testLedger1, Number: "3000", Name: "eur", Class: valueobject.ClassAsset, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1},
	}
}

func TestBelongsToSubLedger(t *testing.T) {
	t.Parallel()
	accounts := periodAccounts()
	usdEntry := entity.Entry{ID: "e-1", PostingID: "p-1", AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10, AssetCode: testUSD, AccountSeq: 1}
	usdKey := service.Key(testTenant1, testLedger1, testUSD)
	eurKey := service.Key(testTenant1, testLedger1, testEUR)
	if !service.BelongsToSubLedger(usdEntry, accounts, usdKey) {
		t.Error("USD entry must belong to USD key")
	}
	if service.BelongsToSubLedger(usdEntry, accounts, eurKey) {
		t.Error("USD entry must not match EUR key (unlike assets never summed)")
	}
	if service.BelongsToSubLedger(usdEntry, accounts, service.Key(testTenant2, testLedger1, testUSD)) {
		t.Error("wrong tenant must not match")
	}
	if service.BelongsToSubLedger(entity.Entry{AccountID: "ghost"}, accounts, usdKey) {
		t.Error("unknown account must not match")
	}
	// Asset mismatch between entry and its account never matches.
	mismatch := usdEntry
	mismatch.AssetCode = testEUR
	if service.BelongsToSubLedger(mismatch, accounts, eurKey) {
		t.Error("entry/account asset mismatch must not match")
	}
	if !usdKey.Matches(testTenant1, testLedger1, testUSD) || usdKey.Matches(testTenant1, testLedger1, testEUR) {
		t.Error("Matches wrong")
	}
}

func openPeriodData() entity.PeriodData {
	return entity.PeriodData{
		ID: "pd-1", TenantID: testTenant1, LedgerID: testLedger1,
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC", Status: entity.PeriodOpen, Version: 1,
	}
}

func openingLines() []service.OpeningBalanceLine {
	return []service.OpeningBalanceLine{
		{AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD},
		{AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD},
	}
}

func openingEvidence() service.EvidenceRef {
	return service.EvidenceRef{Source: "migration", URI: "s3://ledger/opening.csv", ApprovedBy: "cfo"}
}

func TestValidateOpeningBalance(t *testing.T) {
	t.Parallel()
	if err := service.ValidateOpeningBalance(openPeriodData(), openingLines(), openingEvidence()); err != nil {
		t.Fatalf("valid import: %v", err)
	}
	closed := openPeriodData()
	closed.Status = entity.PeriodClosed
	if err := service.ValidateOpeningBalance(closed, openingLines(), openingEvidence()); err == nil {
		t.Error("closed period must fail")
	}
	if err := service.ValidateOpeningBalance(openPeriodData(), openingLines(), service.EvidenceRef{}); err == nil {
		t.Error("missing evidence must fail")
	}
	bad := openingLines()
	bad[1].AmountMinor = 90
	if err := service.ValidateOpeningBalance(openPeriodData(), bad, openingEvidence()); err == nil {
		t.Error("unbalanced lines must fail")
	}
	if err := service.ValidateOpeningBalance(openPeriodData(), nil, openingEvidence()); err == nil {
		t.Error("empty lines must fail")
	}
	bad = openingLines()
	bad[0].AmountMinor = 0
	if err := service.ValidateOpeningBalance(openPeriodData(), bad, openingEvidence()); err == nil {
		t.Error("zero line must fail")
	}
	bad = openingLines()
	bad[0].Side = "SIDEWAYS"
	if err := service.ValidateOpeningBalance(openPeriodData(), bad, openingEvidence()); err == nil {
		t.Error("bad side must fail")
	}
}
