package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testUSD      = "USD"
	testAccount1 = "40000000-0000-4000-8000-000000000001"
	testAccount2 = "40000000-0000-4000-8000-000000000002"
	testPosting1 = "50000000-0000-4000-8000-000000000001"
	testPosting2 = "50000000-0000-4000-8000-000000000002"
	testTenantID = "10000000-0000-4000-8000-000000000001"
	testLedgerID = "20000000-0000-4000-8000-000000000001"
	testPeriod1  = "60000000-0000-4000-8000-000000000001"
	testTransfer = "transfer.v1"
)

// sliceIDs mints deterministic entry IDs for the slice reversal.
type sliceIDs struct{ n int }

func (s *sliceIDs) NewID() string {
	s.n++
	return fmt.Sprintf("22222222-2222-4222-8222-%012x", s.n)
}

func setupSliceAccounts(t *testing.T, at time.Time) (aggregate.Account, aggregate.Account, map[valueobject.AccountID]entity.AccountData) {
	t.Helper()
	src, err := aggregate.OpenAccount(aggregate.OpenAccountParams{
		ID: testAccount1, TenantID: testTenantID, LedgerID: testLedgerID, Number: "2001", Name: "Source",
		Class: valueobject.ClassLiability, AssetCode: testUSD, EventID: "ev-0", OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenAccount src: %v", err)
	}
	dst, err := aggregate.OpenAccount(aggregate.OpenAccountParams{
		ID: testAccount2, TenantID: testTenantID, LedgerID: testLedgerID, Number: "2002", Name: "Dest",
		Class: valueobject.ClassLiability, AssetCode: testUSD, EventID: "ev-0b", OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenAccount dst: %v", err)
	}
	accounts := map[valueobject.AccountID]entity.AccountData{
		testAccount1: src.Record(), testAccount2: dst.Record(),
	}
	return src, dst, accounts
}

func constructSlicePosting(t *testing.T, accounts map[valueobject.AccountID]entity.AccountData, at time.Time) aggregate.Posting {
	t.Helper()
	posting, err := aggregate.ConstructPosting(aggregate.PostingParams{
		ID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID, Operation: testTransfer,
		Entries: []entity.Entry{
			{ID: "30000000-0000-4000-8000-000000000001", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 5000, AssetCode: testUSD, AccountSeq: 1},
			{ID: "30000000-0000-4000-8000-000000000002", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 5000, AssetCode: testUSD, AccountSeq: 1},
		},
		Accounts: accounts, EffectiveAt: at, RecordedAt: at, EventID: "ev-1",
	})
	if err != nil {
		t.Fatalf("ConstructPosting: %v", err)
	}
	return posting
}

func verifySliceSpecifications(t *testing.T, ctx context.Context, posting aggregate.Posting, accounts map[valueobject.AccountID]entity.AccountData, at time.Time) {
	t.Helper()
	template := specification.PostingTemplate{Operation: testTransfer, Version: "v1", Rules: []specification.TemplateRule{
		{Class: valueobject.ClassLiability, Side: valueobject.DirectionDebit},
		{Class: valueobject.ClassLiability, Side: valueobject.DirectionCredit},
	}}
	rec := posting.Record()
	period := entity.PeriodData{ID: testPeriod1, TenantID: testTenantID, LedgerID: testLedgerID,
		Start: at.Add(-time.Hour), End: at.Add(time.Hour), Timezone: "UTC",
		Status: entity.PeriodOpen, Version: 1}
	checks := specification.All[entity.PostingData](
		specification.PostingBalancesPerCurrency(),
		specification.PostingTemplateAllowed(template, accounts),
	)
	if res := checks.Evaluate(ctx, rec); !res.Passed() {
		t.Fatalf("posting specs: %+v", res)
	}
	if res := specification.AccountActive().Evaluate(ctx, accounts[testAccount1]); !res.Passed() {
		t.Fatalf("source active: %+v", res)
	}
	if res := specification.PeriodOpen().Evaluate(ctx, period); !res.Passed() {
		t.Fatalf("period open: %+v", res)
	}
	funds := specification.SufficientFunds(specification.BalanceSnapshot{Available: valueobject.MustMoney(9000, testUSD)})
	if res := funds.Evaluate(ctx, valueobject.MustMoney(5000, testUSD)); !res.Passed() {
		t.Fatalf("funds: %+v", res)
	}
}

func verifySliceEvents(t *testing.T, src, dst aggregate.Account, posting, rev aggregate.Posting) {
	t.Helper()
	got := map[string]int{}
	for _, a := range []*aggregate.Account{&src, &dst} {
		for _, e := range a.UncommittedEvents() {
			got[e.EventType()]++
		}
	}
	for _, e := range posting.UncommittedEvents() {
		got[e.EventType()]++
	}
	for _, e := range rev.UncommittedEvents() {
		got[e.EventType()]++
	}
	want := map[string]int{
		"account.created.v1":      2,
		"transaction.posted.v1":   2,
		"transaction.reversed.v1": 1,
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("event %s count = %d, want %d (all = %v)", k, got[k], n, got)
		}
	}
}

// TestLedgerSlice composes the whole domain vertical without infrastructure:
// open accounts, construct a balanced posting, evaluate the spec composition
// (active + balanced + template + period + funds), reverse, and assert the
// emitted event chain.
func TestLedgerSlice(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	at := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)

	src, dst, accounts := setupSliceAccounts(t, at)
	posting := constructSlicePosting(t, accounts, at)
	verifySliceSpecifications(t, ctx, posting, accounts, at)

	rev, err := aggregate.ReversePosting(posting, accounts, aggregate.ReverseParams{
		NewID: testPosting2, Reason: "slice test", Actor: "u-1", EventID: "ev-2", At: at, IDGen: &sliceIDs{},
	})
	if err != nil {
		t.Fatalf("ReversePosting: %v", err)
	}

	verifySliceEvents(t, src, dst, posting, rev)
}
