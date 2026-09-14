package specification_test

import (
	"context"
	"testing"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func txEntries(pid valueobject.PostingID, dAmt, cAmt int64) []entity.Entry {
	return []entity.Entry{
		{ID: "e-1", PostingID: pid, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: dAmt, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-2", PostingID: pid, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: cAmt, AssetCode: testUSD, AccountSeq: 1},
	}
}

func txPosting(pid valueobject.PostingID, entries []entity.Entry) entity.PostingData {
	return entity.PostingData{ID: pid, TenantID: testTenant1, LedgerID: testLedger1, Operation: "transfer.v1",
		Entries: entries}
}

func TestEntryAmountPositive(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.EntryAmountPositive().Evaluate(ctx, txEntries(testPosting1, 5, 5)[0]); !res.Passed() {
		t.Fatal("positive entry must pass")
	}
	res := specification.EntryAmountPositive().Evaluate(ctx, txEntries(testPosting2, 0, 0)[0])
	if res.Passed() || res.Violations[0].Code != "INVALID_ENTRY_AMOUNT" {
		t.Fatalf("zero entry = %+v", res)
	}
	var zero entity.Entry
	if res := specification.EntryAmountPositive().Evaluate(ctx, zero); res.Passed() {
		t.Fatal("zero entry must fail, not panic")
	}
}

func TestPostingBalancesPerCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.PostingBalancesPerCurrency().Evaluate(ctx, txPosting(testPosting1, txEntries(testPosting1, 100, 100))); !res.Passed() {
		t.Fatalf("balanced must pass: %+v", res)
	}
	res := specification.PostingBalancesPerCurrency().Evaluate(ctx, txPosting(testPosting2, txEntries(testPosting2, 100, 90)))
	if res.Passed() {
		t.Fatal("imbalanced must fail")
	}
	v := res.Violations[0]
	if v.Code != "UNBALANCED_TRANSACTION" || v.Details["debits"] != "100" || v.Details["credits"] != "90" || v.Details["asset"] != testUSD {
		t.Fatalf("violation = %+v", v)
	}
	// Multi-asset: broken EUR lot alongside balanced USD lot.
	entries := append(txEntries(testPosting3, 100, 100),
		entity.Entry{ID: "e-3", PostingID: testPosting3, AccountID: testAccount3, Side: valueobject.DirectionDebit, AmountMinor: 50, AssetCode: testEUR, AccountSeq: 1},
		entity.Entry{ID: "e-4", PostingID: testPosting3, AccountID: "a-4", Side: valueobject.DirectionCredit, AmountMinor: 40, AssetCode: testEUR, AccountSeq: 1},
	)
	res = specification.PostingBalancesPerCurrency().Evaluate(ctx, txPosting(testPosting3, entries))
	if res.Passed() || len(res.Violations) != 1 || res.Violations[0].Details["asset"] != testEUR {
		t.Fatalf("multi-asset = %+v", res)
	}
}

func transferTemplate() specification.PostingTemplate {
	return specification.PostingTemplate{
		Operation: "transfer.v1", Version: "v1",
		Rules: []specification.TemplateRule{
			{Class: valueobject.ClassLiability, Side: valueobject.DirectionDebit},
			{Class: valueobject.ClassLiability, Side: valueobject.DirectionCredit},
		},
	}
}

func templateAccounts() map[valueobject.AccountID]entity.AccountData {
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: {ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "2001", Name: "src", Class: valueobject.ClassLiability, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
		testAccount2: {ID: testAccount2, TenantID: testTenant1, LedgerID: testLedger1, Number: "2002", Name: "dst", Class: valueobject.ClassLiability, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1},
	}
}

func TestPostingTemplateAllowed(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	spec := specification.PostingTemplateAllowed(transferTemplate(), templateAccounts())
	if res := spec.Evaluate(ctx, txPosting(testPosting1, txEntries(testPosting1, 100, 100))); !res.Passed() {
		t.Fatalf("matching template must pass: %+v", res)
	}
	badOp := txPosting(testPosting2, txEntries(testPosting2, 100, 100))
	badOp.Operation = "payout.v1"
	if res := spec.Evaluate(ctx, badOp); res.Passed() || res.Violations[0].Code != "INVALID_POSTING_TEMPLATE" {
		t.Fatalf("operation mismatch = %+v", res)
	}
	accounts := templateAccounts()
	accounts[testAccount1] = entity.AccountData{ID: testAccount1, TenantID: testTenant1, LedgerID: testLedger1, Number: "1000", Name: "cash", Class: valueobject.ClassAsset, AssetCode: testUSD, Status: valueobject.StatusActive, Version: 1}
	assetSpec := specification.PostingTemplateAllowed(transferTemplate(), accounts)
	if res := assetSpec.Evaluate(ctx, txPosting(testPosting3, txEntries(testPosting3, 100, 100))); res.Passed() {
		t.Fatal("asset debit against liability-only template must fail")
	}
	unknownSpec := specification.PostingTemplateAllowed(transferTemplate(), map[valueobject.AccountID]entity.AccountData{})
	if res := unknownSpec.Evaluate(ctx, txPosting(testPosting4, txEntries(testPosting4, 100, 100))); res.Passed() {
		t.Fatal("unknown account must fail")
	}
}

func TestValidCurrency(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	same := specification.CurrencyCheck{EntryAsset: testUSD, AccountAsset: testUSD}
	if res := specification.ValidCurrency().Evaluate(ctx, same); !res.Passed() {
		t.Fatalf("same asset must pass: %+v", res)
	}
	diff := specification.CurrencyCheck{EntryAsset: testUSD, AccountAsset: testEUR}
	if res := specification.ValidCurrency().Evaluate(ctx, diff); res.Passed() || res.Violations[0].Code != "CURRENCY_MISMATCH" {
		t.Fatalf("different assets = %+v", res)
	}
	fx := specification.CurrencyCheck{EntryAsset: testUSD, AccountAsset: testEUR, FXApproved: true}
	if res := specification.ValidCurrency().Evaluate(ctx, fx); !res.Passed() {
		t.Fatalf("approved FX must pass: %+v", res)
	}
}

func TestOriginalExists(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if res := specification.OriginalExists().Evaluate(ctx, specification.OriginalRef{ID: testPosting1, Found: true}); !res.Passed() {
		t.Fatal("found must pass")
	}
	if res := specification.OriginalExists().Evaluate(ctx, specification.OriginalRef{ID: "p-9"}); res.Passed() || res.Violations[0].Code != "ORIGINAL_NOT_FOUND" {
		t.Fatalf("missing = %+v", res)
	}
}
