package specification_test

import (
	"context"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func activeAccount() entity.AccountData {
	return entity.AccountData{ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "1000", Name: "Cash",
		Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1}
}

func TestAccountActiveMatrix(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.AccountActive().Evaluate(ctx, activeAccount()); !res.Passed() {
		t.Fatalf("ACTIVE must pass: %+v", res)
	}
	frozen := activeAccount()
	frozen.Status = valueobject.StatusFrozen
	res := specification.AccountActive().Evaluate(ctx, frozen)
	if res.Passed() || res.Violations[0].Code != "ACCOUNT_FROZEN" {
		t.Fatalf("FROZEN = %+v", res)
	}
	closed := activeAccount()
	closed.Status = valueobject.StatusClosed
	res = specification.AccountActive().Evaluate(ctx, closed)
	if res.Passed() || res.Violations[0].Code != "ACCOUNT_CLOSED" {
		t.Fatalf("CLOSED = %+v", res)
	}
	unknown := activeAccount()
	unknown.Status = "WEIRD"
	if res := specification.AccountActive().Evaluate(ctx, unknown); res.Passed() {
		t.Fatal("unknown status must fail")
	}
	var zero entity.AccountData
	if res := specification.AccountActive().Evaluate(ctx, zero); res.Passed() {
		t.Fatal("zero account must fail, not panic")
	}
}

func TestSameTenant(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.SameTenant().Evaluate(ctx, specification.TenantPair{A: testTenant1, B: testTenant1}); !res.Passed() {
		t.Fatalf("same tenant must pass: %+v", res)
	}
	res := specification.SameTenant().Evaluate(ctx, specification.TenantPair{A: testTenant1, B: "t-2"})
	if res.Passed() || res.Violations[0].Code != "TENANT_MISMATCH" {
		t.Fatalf("different tenants = %+v", res)
	}
	if res := specification.SameTenant().Evaluate(ctx, specification.TenantPair{}); res.Passed() {
		t.Fatal("empty tenants must fail")
	}
}
