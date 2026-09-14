package aggregate_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// stubIDs is a deterministic valueobject.IDGenerator for reversal entry IDs.
type stubIDs struct{ n int }

func (s *stubIDs) NewID() string {
	s.n++
	return fmt.Sprintf("11111111-1111-4111-8111-%012x", s.n)
}

func postingAccounts() map[valueobject.AccountID]entity.AccountData {
	mk := func(id valueobject.AccountID, number string, class valueobject.AccountClass, status valueobject.AccountStatus) entity.AccountData {
		return entity.AccountData{
			ID: id, TenantID: testTenantID, LedgerID: testLedgerID, Number: number, Name: number,
			Class: class, AssetCode: testUSD, Status: status, Version: 1,
		}
	}
	return map[valueobject.AccountID]entity.AccountData{
		testAccount1: mk(testAccount1, "1000", valueobject.ClassAsset, valueobject.StatusActive),
		testAccount2: mk(testAccount2, "2000", valueobject.ClassLiability, valueobject.StatusActive),
	}
}

func postingEntries(pid valueobject.PostingID, dAmt, cAmt int64) []entity.Entry {
	return []entity.Entry{
		{ID: "e-1", PostingID: pid, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: dAmt, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-2", PostingID: pid, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: cAmt, AssetCode: testUSD, AccountSeq: 1},
	}
}

func postingParams(pid valueobject.PostingID, entries []entity.Entry, accounts map[valueobject.AccountID]entity.AccountData) aggregate.PostingParams {
	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	return aggregate.PostingParams{
		ID: pid, TenantID: testTenantID, LedgerID: testLedgerID, Operation: "transfer.v1",
		Description: "test", Entries: entries, Accounts: accounts,
		EffectiveAt: at, RecordedAt: at, EventID: testEvent1,
	}
}

func TestConstructPosting(t *testing.T) {
	t.Parallel()
	p, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 250, 250), postingAccounts()))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	if len(p.Record().Entries) != 2 {
		t.Fatalf("entries = %d", len(p.Record().Entries))
	}
	evts := p.UncommittedEvents()
	if len(evts) != 1 || evts[0].EventType() != "transaction.posted.v1" {
		t.Fatalf("events = %v", evts)
	}
	p.ClearEvents()
	if len(p.UncommittedEvents()) != 0 {
		t.Fatal("ClearEvents must drain")
	}
}

func TestUnbalancedFailsWithTotals(t *testing.T) {
	t.Parallel()
	_, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 90), postingAccounts()))
	if err == nil {
		t.Fatal("unbalanced posting must fail")
	}
	if !strings.Contains(err.Error(), "UNBALANCED_TRANSACTION") {
		t.Fatalf("err = %v, want UNBALANCED_TRANSACTION", err)
	}
	if !strings.Contains(err.Error(), "100") || !strings.Contains(err.Error(), "90") {
		t.Fatalf("err = %v, want both totals in details", err)
	}
}

func TestCrossCurrencyLots(t *testing.T) {
	t.Parallel()
	accounts := postingAccounts()
	accounts[testAccount3] = entity.AccountData{ID: testAccount3, TenantID: testTenantID, LedgerID: testLedgerID, Number: "3000", Name: "eur", Class: valueobject.ClassAsset, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
	accounts[testAccount4] = entity.AccountData{ID: testAccount4, TenantID: testTenantID, LedgerID: testLedgerID, Number: "4000", Name: "fx", Class: valueobject.ClassLiability, AssetCode: testEUR, Status: valueobject.StatusActive, Version: 1}
	entries := []entity.Entry{
		{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 10850, AssetCode: testUSD, AccountSeq: 1},
		{ID: "e-3", PostingID: testPosting1, AccountID: testAccount3, Side: valueobject.DirectionDebit, AmountMinor: 10000, AssetCode: testEUR, AccountSeq: 1},
		{ID: "e-4", PostingID: testPosting1, AccountID: testAccount4, Side: valueobject.DirectionCredit, AmountMinor: 10000, AssetCode: testEUR, AccountSeq: 1},
	}
	params := postingParams(testPosting1, entries, accounts)
	params.Metadata = map[string]string{"fx_trade_id": "fx-1"}
	if _, err := aggregate.ConstructPosting(params); err != nil {
		t.Fatalf("balanced FX lots must pass: %v", err)
	}
	// Break one lot: EUR credits 9999.
	entries[3].AmountMinor = 9999
	if _, err := aggregate.ConstructPosting(postingParams(testPosting1, entries, accounts)); err == nil {
		t.Fatal("broken EUR lot must fail")
	}
}

func TestFrozenClosedRejectPostings(t *testing.T) {
	t.Parallel()
	accounts := postingAccounts()
	frozen := accounts[testAccount1]
	frozen.Status = valueobject.StatusFrozen
	accounts[testAccount1] = frozen
	if _, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)); err == nil || !strings.Contains(err.Error(), "ACCOUNT_FROZEN") {
		t.Fatalf("frozen err = %v", err)
	}
	closed := accounts[testAccount1]
	closed.Status = valueobject.StatusClosed
	accounts[testAccount1] = closed
	if _, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts)); err == nil || !strings.Contains(err.Error(), "ACCOUNT_CLOSED") {
		t.Fatalf("closed err = %v", err)
	}
}

func testReversePostingValidation(t *testing.T, orig aggregate.Posting, accounts map[valueobject.AccountID]entity.AccountData, at time.Time) {
	t.Helper()
	if _, err := aggregate.ReversePosting(orig, accounts, aggregate.ReverseParams{NewID: testPosting3, Actor: testUser1, EventID: testEvent3, At: at, IDGen: &stubIDs{}}); err == nil {
		t.Fatal("reversal without reason must fail")
	}
	if _, err := aggregate.ReversePosting(orig, accounts, aggregate.ReverseParams{NewID: testPosting3, Reason: "x", Actor: testUser1, EventID: testEvent3, At: at}); err == nil {
		t.Fatal("reversal without generator must fail closed")
	}
}

func TestReversePosting(t *testing.T) {
	t.Parallel()
	accounts := postingAccounts()
	orig, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), accounts))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	at := time.Date(2026, 9, 14, 8, 0, 0, 0, time.UTC)
	rev, err := aggregate.ReversePosting(orig, accounts, aggregate.ReverseParams{
		NewID: testPosting2, Reason: "entry error", Actor: testUser1, EventID: testEvent2, At: at, IDGen: &stubIDs{},
	})
	if err != nil {
		t.Fatalf("ReversePosting: %v", err)
	}
	rec := rev.Record()
	if rec.ReversalOf == nil || *rec.ReversalOf != testPosting1 {
		t.Fatalf("ReversalOf = %v", rec.ReversalOf)
	}
	if rec.Entries[0].Side != valueobject.DirectionCredit || rec.Entries[1].Side != valueobject.DirectionDebit {
		t.Fatalf("reversal sides not mirrored: %+v", rec.Entries)
	}
	if len(rev.UncommittedEvents()) != 2 {
		t.Fatalf("reversal must carry posted + reversed events, got %d", len(rev.UncommittedEvents()))
	}
	last := rev.UncommittedEvents()[1]
	if last.EventType() != "transaction.reversed.v1" {
		t.Fatalf("last event = %s", last.EventType())
	}
	// Original unchanged: still one event, no ReversalOf.
	if orig.Record().ReversalOf != nil || len(orig.UncommittedEvents()) != 1 {
		t.Fatal("original posting must be unchanged")
	}
	testReversePostingValidation(t, orig, accounts, at)
}

func TestPostingImmutable(t *testing.T) {
	t.Parallel()
	typ := reflect.TypeOf(aggregate.Posting{})
	for i := 0; i < typ.NumField(); i++ {
		if typ.Field(i).IsExported() {
			t.Errorf("Posting exposes mutable field %s", typ.Field(i).Name)
		}
	}
	ptr := reflect.PointerTo(typ)
	for i := 0; i < ptr.NumMethod(); i++ {
		n := ptr.Method(i).Name
		if strings.HasPrefix(n, "Set") || strings.HasPrefix(n, "Update") || strings.HasPrefix(n, "Delete") || strings.HasPrefix(n, "Mutate") {
			t.Errorf("Posting has mutator %s", n)
		}
	}
	// Record copies isolate the aggregate.
	p, err := aggregate.ConstructPosting(postingParams(testPosting1, postingEntries(testPosting1, 100, 100), postingAccounts()))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	rec := p.Record()
	rec.Entries[0].AmountMinor = 1
	if p.Record().Entries[0].AmountMinor != 100 {
		t.Fatal("Record must return a copy")
	}
}

func TestPostingIDAccessor(t *testing.T) {
	t.Parallel()
	p, err := aggregate.ConstructPosting(postingParams(testPosting7, postingEntries(testPosting7, 500, 500), postingAccounts()))
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	if p.ID() != testPosting7 {
		t.Fatalf("ID = %s", p.ID())
	}
}
