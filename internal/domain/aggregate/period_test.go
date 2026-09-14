package aggregate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func openTestPeriod(t *testing.T) *aggregate.Period {
	t.Helper()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	p, err := aggregate.OpenPeriod(aggregate.OpenPeriodParams{
		ID: testPeriod1, TenantID: testTenantID, LedgerID: testLedgerID, Start: start, End: end,
		Timezone: "UTC", Actor: testUser1, EventID: testEvent0, OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenPeriod: %v", err)
	}
	return &p
}

func testPeriodCloseAndDoubleClose(t *testing.T, p *aggregate.Period, at time.Time) {
	t.Helper()
	if err := p.Close(aggregate.ClosePeriodParams{Actor: testUser1, EventID: testEvent1, OccurredAt: at}); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if rec := p.Record(); rec.Status != entity.PeriodClosed || rec.Version != 2 {
		t.Fatalf("record = %+v", rec)
	}
	if err := p.Close(aggregate.ClosePeriodParams{Actor: testUser1, EventID: testEvent2, OccurredAt: at}); err == nil || !strings.Contains(err.Error(), "PERIOD_CLOSED") {
		t.Fatalf("double close err = %v", err)
	}
}

func testPeriodReopenCycle(t *testing.T, p *aggregate.Period, at time.Time) {
	t.Helper()
	if err := p.Reopen(aggregate.ReopenPeriodParams{Actor: "admin", EventID: testEvent3, OccurredAt: at}); err == nil || !strings.Contains(err.Error(), "PERIOD_REOPEN_APPROVAL") {
		t.Fatalf("reopen without approval err = %v", err)
	}
	if err := p.Reopen(aggregate.ReopenPeriodParams{ApprovedBy: "cfo", Reason: "late fee", Actor: "admin", EventID: testEvent3, OccurredAt: at}); err != nil {
		t.Fatalf("Reopen: %v", err)
	}
	if err := p.Close(aggregate.ClosePeriodParams{Actor: testUser1, EventID: testEvent4, OccurredAt: at}); err != nil {
		t.Fatalf("Close again: %v", err)
	}
}

func TestPeriodLifecycle(t *testing.T) {
	t.Parallel()
	p := openTestPeriod(t)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	testPeriodCloseAndDoubleClose(t, p, at)
	testPeriodReopenCycle(t, p, at)
	want := []string{"period.opened.v1", "period.closed.v1", "period.reopened.v1", "period.closed.v1"}
	evts := p.UncommittedEvents()
	if len(evts) != len(want) {
		t.Fatalf("events = %d, want %d", len(evts), len(want))
	}
	for i, w := range want {
		if evts[i].EventType() != w || evts[i].AggregateID() != testPeriod1 {
			t.Errorf("event[%d] = %s %s", i, evts[i].EventType(), evts[i].AggregateID())
		}
	}
	p.ClearEvents()
	if len(p.UncommittedEvents()) != 0 {
		t.Fatal("ClearEvents must drain")
	}
}

func TestPeriodCloseBlocked(t *testing.T) {
	t.Parallel()
	p := openTestPeriod(t)
	at := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	err := p.Close(aggregate.ClosePeriodParams{UnresolvedWorkflows: 1, UnresolvedBreaks: 2, Actor: testUser1, EventID: testEvent9, OccurredAt: at})
	if err == nil || !strings.Contains(err.Error(), "PERIOD_CLOSE_BLOCKED") {
		t.Fatalf("err = %v", err)
	}
	if p.Record().Status != entity.PeriodOpen || len(p.UncommittedEvents()) != 1 {
		t.Fatal("blocked close must leave state and events untouched")
	}
}

func TestPeriodReopenOnOpenFails(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)

	type testCase struct {
		name          string
		params        aggregate.ReopenPeriodParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name: "reopen open period fails",
			params: aggregate.ReopenPeriodParams{
				ApprovedBy: "cfo",
				Reason:     "x",
				Actor:      "a",
				EventID:    "ev-9",
				OccurredAt: at,
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := openTestPeriod(t)
			err := p.Reopen(tc.params)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestOpenPeriodValidation(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	at := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	base := aggregate.OpenPeriodParams{
		ID:         "pd-9",
		TenantID:   testTenantID,
		LedgerID:   testLedgerID,
		Start:      start,
		End:        end,
		Timezone:   "UTC",
		Actor:      testUser1,
		EventID:    testEvent0,
		OccurredAt: at,
	}

	type testCase struct {
		name          string
		params        aggregate.OpenPeriodParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "valid period params pass",
			params:        base,
			expectedError: false,
		},
		{
			name: "inverted bounds rejected",
			params: func() aggregate.OpenPeriodParams {
				p := base
				p.Start, p.End = end, start
				return p
			}(),
			expectedError: true,
		},
		{
			name: "empty timezone rejected",
			params: func() aggregate.OpenPeriodParams {
				p := base
				p.Timezone = ""
				return p
			}(),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := aggregate.OpenPeriod(tc.params)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
