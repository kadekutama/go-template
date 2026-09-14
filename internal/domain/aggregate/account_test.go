package aggregate_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

const (
	testTenantID = "t-1"
	testLedgerID = "l-1"
	testUSD      = "USD"
	testEUR      = "EUR"
	testUser1    = "u-1"
	testAccount1 = "a-1"
	testAccount2 = "a-2"
	testAccount3 = "a-3"
	testAccount4 = "a-4"
	testPosting1 = "p-1"
	testPosting2 = "p-2"
	testPosting3 = "p-3"
	testPosting7 = "p-7"
	testEvent0   = "ev-0"
	testEvent1   = "ev-1"
	testEvent2   = "ev-2"
	testEvent3   = "ev-3"
	testEvent4   = "ev-4"
	testEvent9   = "ev-9"
	testPeriod1  = "pd-1"
)

func openTestAccount(t *testing.T, id valueobject.AccountID) *aggregate.Account {
	t.Helper()
	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)
	a, err := aggregate.OpenAccount(aggregate.OpenAccountParams{
		ID: id, TenantID: testTenantID, LedgerID: testLedgerID, Number: "1000", Name: "Cash",
		Class: valueobject.ClassAsset, AssetCode: testUSD, Purpose: "ops",
		Metadata: map[string]string{"k": "v"}, OpenedBy: testUser1,
		EventID: testEvent0, OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	return &a
}

func TestOpenAccount(t *testing.T) {
	t.Parallel()
	a := openTestAccount(t, testAccount1)
	rec := a.Record()
	if rec.Status != valueobject.StatusActive || rec.Version != 1 {
		t.Fatalf("record = %+v", rec)
	}
	evts := a.UncommittedEvents()
	if len(evts) != 1 || evts[0].EventType() != "account.created.v1" {
		t.Fatalf("events = %v", evts)
	}
	if evts[0].AggregateID() != testAccount1 || evts[0].AggregateVersion() != 1 {
		t.Fatalf("event envelope = %+v", evts[0])
	}
	a.ClearEvents()
	if len(a.UncommittedEvents()) != 0 {
		t.Fatal("ClearEvents must drain")
	}
}

func TestLifecycleEmissionOrder(t *testing.T) {
	t.Parallel()
	a := openTestAccount(t, testAccount1)
	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	tr := func(ev string) aggregate.TransitionParams {
		return aggregate.TransitionParams{Actor: testUser1, EventID: ev, OccurredAt: at}
	}
	if err := a.Freeze(aggregate.FreezeParams{TransitionParams: tr(testEvent1), Reason: "review"}); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if err := a.Unfreeze(tr(testEvent2)); err != nil {
		t.Fatalf("Unfreeze: %v", err)
	}
	if err := a.Freeze(aggregate.FreezeParams{TransitionParams: tr(testEvent3), Reason: "again"}); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if err := a.Close(aggregate.CloseParams{TransitionParams: tr(testEvent4), Reason: "obsolete"}); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if v := a.Record().Version; v != 5 {
		t.Fatalf("version = %d, want 5", v)
	}
	want := []string{"account.created.v1", "account.frozen.v1", "account.unfrozen.v1", "account.frozen.v1", "account.closed.v1"}
	evts := a.UncommittedEvents()
	if len(evts) != len(want) {
		t.Fatalf("events = %d, want %d", len(evts), len(want))
	}
	for i, w := range want {
		if evts[i].EventType() != w {
			t.Errorf("event[%d] = %s, want %s", i, evts[i].EventType(), w)
		}
		if evts[i].AggregateID() != testAccount1 {
			t.Errorf("event[%d] aggregate = %s", i, evts[i].AggregateID())
		}
	}
}

func testClosedAccountOperations(t *testing.T, a *aggregate.Account, tr aggregate.TransitionParams) {
	t.Helper()
	for _, err := range []error{
		a.Freeze(aggregate.FreezeParams{TransitionParams: tr, Reason: "x"}),
		a.Unfreeze(tr),
		a.Close(aggregate.CloseParams{TransitionParams: tr, Reason: "x"}),
		a.Reparent(aggregate.ReparentParams{TransitionParams: tr}),
		a.UpdateDetails(aggregate.UpdateDetailsParams{TransitionParams: tr, Name: "n"}),
	} {
		if err == nil || !strings.Contains(err.Error(), "ACCOUNT_CLOSED") {
			t.Errorf("closed mutation err = %v, want ACCOUNT_CLOSED", err)
		}
	}
}

func TestIllegalTransitions(t *testing.T) {
	t.Parallel()
	a := openTestAccount(t, testAccount1)
	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	tr := aggregate.TransitionParams{Actor: testUser1, EventID: testEvent9, OccurredAt: at}
	if err := a.Unfreeze(tr); err == nil || !strings.Contains(err.Error(), "ACCOUNT_NOT_FROZEN") {
		t.Errorf("Unfreeze ACTIVE err = %v", err)
	}
	if err := a.Freeze(aggregate.FreezeParams{TransitionParams: tr}); err == nil || !strings.Contains(err.Error(), "ACCOUNT_REASON_REQUIRED") {
		t.Errorf("Freeze without reason err = %v", err)
	}
	if err := a.Freeze(aggregate.FreezeParams{TransitionParams: tr, Reason: "r"}); err != nil {
		t.Fatalf("Freeze: %v", err)
	}
	if err := a.Freeze(aggregate.FreezeParams{TransitionParams: tr, Reason: "r"}); err == nil || !strings.Contains(err.Error(), "ACCOUNT_ALREADY_FROZEN") {
		t.Errorf("double Freeze err = %v", err)
	}
	n := len(a.UncommittedEvents())
	if err := a.Close(aggregate.CloseParams{TransitionParams: tr, Reason: "done"}); err != nil {
		t.Fatalf("Close: %v", err)
	}
	testClosedAccountOperations(t, a, tr)
	if len(a.UncommittedEvents()) != n+1 {
		t.Error("failed mutations must not emit events or bump version")
	}
	if v := a.Record().Version; v != 3 {
		t.Errorf("version = %d, want 3", v)
	}
}

func TestReparentAndDetails(t *testing.T) {
	t.Parallel()
	a := openTestAccount(t, "a-1")
	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	tr := aggregate.TransitionParams{Actor: "u-1", EventID: "ev-9", OccurredAt: at}
	parent := valueobject.AccountID("p-1")
	if err := a.Reparent(aggregate.ReparentParams{TransitionParams: tr, NewParent: &parent}); err != nil {
		t.Fatalf("Reparent: %v", err)
	}
	if got := a.Record().ParentID; got == nil || *got != parent {
		t.Fatalf("parent = %v", got)
	}
	self := valueobject.AccountID("a-1")
	if err := a.Reparent(aggregate.ReparentParams{TransitionParams: tr, NewParent: &self}); err == nil {
		t.Error("self-parent must error")
	}
	md := map[string]string{"k": "v2"}
	if err := a.UpdateDetails(aggregate.UpdateDetailsParams{TransitionParams: tr, Name: "Cash+", Purpose: "p", Metadata: md}); err != nil {
		t.Fatalf("UpdateDetails: %v", err)
	}
	md["k"] = "mutated"
	if got := a.Record().Metadata["k"]; got != "v2" {
		t.Fatalf("metadata leaked caller mutation: %q", got)
	}
	if err := a.UpdateDetails(aggregate.UpdateDetailsParams{TransitionParams: tr}); err == nil {
		t.Error("empty name must error")
	}
	types := map[string]bool{}
	for _, e := range a.UncommittedEvents() {
		types[e.EventType()] = true
	}
	if !types["account.updated.v1"] {
		t.Error("expected account.updated.v1 emissions")
	}
}

func TestNormalSideTable(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		class          valueobject.AccountClass
		expectedResult valueobject.Direction
	}

	testCases := []testCase{
		{
			name:           "asset normal side is debit",
			class:          valueobject.ClassAsset,
			expectedResult: valueobject.DirectionDebit,
		},
		{
			name:           "expense normal side is debit",
			class:          valueobject.ClassExpense,
			expectedResult: valueobject.DirectionDebit,
		},
		{
			name:           "liability normal side is credit",
			class:          valueobject.ClassLiability,
			expectedResult: valueobject.DirectionCredit,
		},
		{
			name:           "equity normal side is credit",
			class:          valueobject.ClassEquity,
			expectedResult: valueobject.DirectionCredit,
		},
		{
			name:           "revenue normal side is credit",
			class:          valueobject.ClassRevenue,
			expectedResult: valueobject.DirectionCredit,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expectedResult, tc.class.NormalSide())
		})
	}
}

func TestNoBalanceOrDeposit(t *testing.T) {
	t.Parallel()
	accType := reflect.TypeOf(aggregate.Account{})
	for i := 0; i < accType.NumField(); i++ {
		n := strings.ToLower(accType.Field(i).Name)
		if strings.Contains(n, "balance") {
			t.Errorf("aggregate has balance field %s", accType.Field(i).Name)
		}
	}
	recType := reflect.TypeOf(entity.AccountData{})
	for i := 0; i < recType.NumField(); i++ {
		n := strings.ToLower(recType.Field(i).Name)
		if strings.Contains(n, "balance") {
			t.Errorf("record has balance field %s", recType.Field(i).Name)
		}
	}
	for _, m := range []string{"Deposit", "Withdraw", "SetBalance", "Credit", "Debit"} {
		if _, ok := accType.MethodByName(m); ok {
			t.Errorf("aggregate has forbidden method %s", m)
		}
		if _, ok := reflect.PointerTo(accType).MethodByName(m); ok {
			t.Errorf("aggregate pointer has forbidden method %s", m)
		}
	}
}

func TestOpenValidation(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)
	base := aggregate.OpenAccountParams{
		ID:         testAccount1,
		TenantID:   testTenantID,
		LedgerID:   testLedgerID,
		Number:     "1000",
		Name:       "Cash",
		Class:      valueobject.ClassAsset,
		AssetCode:  testUSD,
		EventID:    testEvent0,
		OccurredAt: at,
	}

	type testCase struct {
		name          string
		params        aggregate.OpenAccountParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name: "missing tenant id",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.TenantID = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "missing ledger id",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.LedgerID = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "missing asset code",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.AssetCode = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "missing name",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.Name = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "missing number",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.Number = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "invalid class",
			params: func() aggregate.OpenAccountParams {
				p := base
				p.Class = "NOPE"
				return p
			}(),
			expectedError: true,
		},
		{
			name: "self-parent rejected",
			params: func() aggregate.OpenAccountParams {
				p := base
				id := valueobject.AccountID(testAccount1)
				p.ParentID = &id
				p.ID = testAccount1
				return p
			}(),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := aggregate.OpenAccount(tc.params)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCloseRequiresReason(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)

	type testCase struct {
		name          string
		params        aggregate.CloseParams
		expectedError string
	}

	testCases := []testCase{
		{
			name: "close without reason fails",
			params: aggregate.CloseParams{
				TransitionParams: aggregate.TransitionParams{
					Actor:      testUser1,
					EventID:    testEvent9,
					OccurredAt: at,
				},
				Reason: "",
			},
			expectedError: "ACCOUNT_REASON_REQUIRED",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := openTestAccount(t, "a-9")
			err := a.Close(tc.params)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
			assert.Len(t, a.UncommittedEvents(), 1)
		})
	}
}
