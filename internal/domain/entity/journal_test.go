package entity_test

import (
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func scopePostings() []entity.PostingData {
	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	mk := func(id, tenant, ledger string) entity.PostingData {
		return entity.PostingData{
			ID: valueobject.PostingID(id), TenantID: valueobject.TenantID(tenant), LedgerID: valueobject.LedgerID(ledger),
			Operation: "transfer.v1",
			Entries: []entity.Entry{
				{ID: "e-1", PostingID: valueobject.PostingID(id), AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10, AssetCode: testUSD, AccountSeq: 1},
				{ID: "e-2", PostingID: valueobject.PostingID(id), AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 10, AssetCode: testUSD, AccountSeq: 1},
			},
			EffectiveAt: at, RecordedAt: at,
		}
	}
	return []entity.PostingData{mk(testPosting1, testTenantID, testLedgerID), mk("p-2", testTenantID, testLedgerID)}
}

func TestNewJournal(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	j, err := entity.NewJournal(testJournal1, testTenantID, testLedgerID, testPeriod1, scopePostings(), "daily", map[string]string{"k": "v"}, at)
	if err != nil {
		t.Fatalf("NewJournal: %v", err)
	}
	if len(j.PostingIDs) != 2 || j.PeriodID != testPeriod1 {
		t.Fatalf("journal = %+v", j)
	}
	mixed := scopePostings()
	mixed[1].TenantID = "t-2"
	if _, err := entity.NewJournal(testJournal1, testTenantID, testLedgerID, testPeriod1, mixed, "x", nil, at); err == nil {
		t.Error("mixed tenants must fail")
	}
	mixed = scopePostings()
	mixed[1].LedgerID = "l-2"
	if _, err := entity.NewJournal(testJournal1, testTenantID, testLedgerID, testPeriod1, mixed, "x", nil, at); err == nil {
		t.Error("mixed ledgers must fail")
	}
	if _, err := entity.NewJournal(testJournal1, testTenantID, testLedgerID, testPeriod1, nil, "x", nil, at); err == nil {
		t.Error("empty postings must fail")
	}
	if _, err := entity.NewJournal(testJournal1, testTenantID, testLedgerID, "", scopePostings(), "x", nil, at); err == nil {
		t.Error("empty period must fail")
	}
	if _, err := entity.NewJournal("", testTenantID, testLedgerID, testPeriod1, scopePostings(), "x", nil, at); err == nil {
		t.Error("empty id must fail")
	}
}

func TestPeriodDataValidate(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	valid := entity.PeriodData{ID: testPeriod1, TenantID: testTenantID, LedgerID: testLedgerID, Start: start, End: end, Timezone: "UTC", Status: entity.PeriodOpen, Version: 1}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	bad := valid
	bad.Start, bad.End = end, start
	if err := bad.Validate(); err == nil {
		t.Error("inverted bounds must fail")
	}
	bad = valid
	bad.Timezone = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty timezone must fail")
	}
	bad = valid
	bad.Status = "AJAR"
	if err := bad.Validate(); err == nil {
		t.Error("bad status must fail")
	}
	bad = valid
	bad.Version = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero version must fail")
	}
	if !valid.Contains(start.Add(time.Hour)) || valid.Contains(end) || valid.Contains(start.Add(-time.Hour)) {
		t.Error("Contains bounds wrong")
	}
}
