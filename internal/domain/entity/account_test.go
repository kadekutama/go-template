package entity_test

import (
	"strings"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testMain       = "Main"
	testLedgerID   = "l-1"
	testTenantID   = "t-1"
	testUSD        = "USD"
	testAccount1   = "a-1"
	testAccount2   = "a-2"
	testPosting1   = "p-1"
	testPeriod1    = "pd-1"
	testJournal1   = "j-1"
	testEmptyAsset = "empty asset"
	testEmptyID    = "empty id"
)

func TestDomainError(t *testing.T) {
	t.Parallel()
	err := entity.NewError("ACCOUNT_FROZEN", "account is frozen")
	if err.Error() != "ACCOUNT_FROZEN: account is frozen" {
		t.Fatalf("Error() = %q", err.Error())
	}
	err = entity.Errorf("E_X", "code %d", 1)
	if err.Code != "E_X" || err.Message != "code 1" {
		t.Fatalf("Errorf = %+v", err)
	}
}

func TestNewLedger(t *testing.T) {
	t.Parallel()
	if _, err := entity.NewLedger(testLedgerID, testTenantID, testMain, testUSD, "v1"); err != nil {
		t.Fatalf("NewLedger: %v", err)
	}
	cases := []struct {
		name                  string
		id, tenant, nm, chart string
		asset                 valueobject.AssetCode
	}{
		{testEmptyID, "", testTenantID, testMain, "v1", testUSD},
		{"empty tenant", testLedgerID, "", testMain, "v1", testUSD},
		{"empty name", testLedgerID, testTenantID, "", "v1", testUSD},
		{testEmptyAsset, testLedgerID, testTenantID, testMain, "v1", ""},
		{"empty chart", testLedgerID, testTenantID, testMain, "", testUSD},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := entity.NewLedger(valueobject.LedgerID(tc.id), valueobject.TenantID(tc.tenant), tc.nm, tc.asset, tc.chart); err == nil {
				t.Error("must error")
			}
		})
	}
}

func TestAccountDataValidate(t *testing.T) {
	t.Parallel()
	valid := entity.AccountData{
		ID: testAccount1, TenantID: testTenantID, LedgerID: testLedgerID, Number: "1000", Name: "Cash",
		Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1,
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	bad := valid
	bad.ID = ""
	if err := bad.Validate(); err == nil || !strings.Contains(err.Error(), "ACCOUNT_ID_REQUIRED") {
		t.Errorf("empty id err = %v", err)
	}
	bad = valid
	bad.TenantID = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty tenant must error")
	}
	bad = valid
	bad.LedgerID = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty ledger must error")
	}
	bad = valid
	bad.Number = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty number must error")
	}
	bad = valid
	bad.Name = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty name must error")
	}
	bad = valid
	bad.Class = "NOPE"
	if err := bad.Validate(); err == nil {
		t.Error("bad class must error")
	}
	bad = valid
	bad.AssetCode = ""
	if err := bad.Validate(); err == nil {
		t.Error("empty asset must error")
	}
	bad = valid
	bad.Status = "PENDING"
	if err := bad.Validate(); err == nil {
		t.Error("bad status must error")
	}
	bad = valid
	bad.Version = 0
	if err := bad.Validate(); err == nil {
		t.Error("zero version must error")
	}
}
