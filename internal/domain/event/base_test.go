package event_test

import (
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/event"
)

const (
	testTenantID    = "t-1"
	testLedgerID    = "l-1"
	testUserID      = "u-1"
	testCorrID      = "corr-1"
	testTraceID     = "tr-1"
	testCmdID       = "cmd-1"
	testUSD         = "USD"
	testAccount1    = "a-1"
	testAccount2    = "a-2"
	testPosting1    = "p-1"
	testPosting2    = "p-2"
	testEvent1      = "ev-1"
	testTransfer1   = "x-1"
	testBatch1      = "b-1"
	testPayment1    = "pi-1"
	testRefund1     = "r-1"
	testPayout1     = "po-1"
	testRun1        = "run-1"
	testBreak1      = "br-1"
	testPeriod1     = "pd-1"
	testFee1        = "f-1"
	testDispute1    = "d-1"
	testTopUp1      = "tp-1"
	testTransferOp  = "transfer.v1"
	testTenant1     = "tn-1"
	testSubject1    = "s-1"
	testFx1         = "fx-1"
	testReport1     = "rep-1"
	testOrigPosting = "p-0"
)

func validArgs() (eventID, aggregateID, aggregateType, eventType string, at time.Time, version, seq int64, meta event.EventMetadata) {
	return "evt-01", "agg-01", "Account", "account.created.v1",
		time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC), 3, 7,
		event.EventMetadata{TenantID: testTenantID, LedgerID: testLedgerID}
}

func verifyBaseAccessors(t *testing.T, evt event.BaseEvent, eventID, aggregateID, aggregateType, eventType string, at time.Time, version, seq int64) {
	t.Helper()
	if evt.EventID() != eventID {
		t.Errorf("EventID = %q, want %q", evt.EventID(), eventID)
	}
	if evt.AggregateID() != aggregateID {
		t.Errorf("AggregateID = %q, want %q", evt.AggregateID(), aggregateID)
	}
	if evt.AggregateType() != aggregateType {
		t.Errorf("AggregateType = %q, want %q", evt.AggregateType(), aggregateType)
	}
	if evt.EventType() != eventType {
		t.Errorf("EventType = %q, want %q", evt.EventType(), eventType)
	}
	if !evt.OccurredAt().Equal(at) || evt.OccurredAt().Location() != time.UTC {
		t.Errorf("OccurredAt = %v, want %v in UTC", evt.OccurredAt(), at)
	}
	if evt.AggregateVersion() != version {
		t.Errorf("AggregateVersion = %d, want %d", evt.AggregateVersion(), version)
	}
	if evt.Sequence() != seq {
		t.Errorf("Sequence = %d, want %d", evt.Sequence(), seq)
	}
}

func verifyBaseMetadata(t *testing.T, got event.EventMetadata) {
	t.Helper()
	if got.TenantID != testTenantID || got.LedgerID != testLedgerID || got.CausationID != testCmdID ||
		got.CorrelationID != testCorrID || got.UserID != testUserID || got.TraceID != testTraceID {
		t.Errorf("Metadata = %+v, want causation/correlation/user/trace preserved", got)
	}
	if got.Custom["k"] != "v" {
		t.Errorf("Metadata.Custom = %v, want map[k:v]", got.Custom)
	}
}

func TestNewBaseEventAccessors(t *testing.T) {
	t.Parallel()
	eventID, aggregateID, aggregateType, eventType, at, version, seq, meta := validArgs()
	meta.CausationID = testCmdID
	meta.CorrelationID = testCorrID
	meta.UserID = testUserID
	meta.TraceID = testTraceID
	meta.Custom = map[string]string{"k": "v"}
	payload := map[string]string{"field": "value"}

	evt, err := event.NewBaseEvent(eventID, aggregateID, aggregateType, eventType, at, version, seq, payload, meta)
	if err != nil {
		t.Fatalf("NewBaseEvent returned error: %v", err)
	}
	var _ event.DomainEvent = evt
	verifyBaseAccessors(t, evt, eventID, aggregateID, aggregateType, eventType, at, version, seq)
	verifyBaseMetadata(t, evt.Metadata())
}

func TestNewBaseEventValidationMatrix(t *testing.T) {
	t.Parallel()
	eventID, aggregateID, aggregateType, eventType, at, version, seq, _ := validArgs()
	custom := event.EventMetadata{TenantID: testTenantID, Custom: map[string]string{"k": "v"}}

	cases := []struct {
		name   string
		mutate func(*string, *string, *string, *string, *time.Time, *int64, *int64, *event.EventMetadata)
	}{
		{"empty event_id", func(eid, _, _, _ *string, _ *time.Time, _, _ *int64, _ *event.EventMetadata) { *eid = "" }},
		{"empty aggregate_id", func(_, aid, _, _ *string, _ *time.Time, _, _ *int64, _ *event.EventMetadata) { *aid = "" }},
		{"empty aggregate_type", func(_, _, aty, _ *string, _ *time.Time, _, _ *int64, _ *event.EventMetadata) { *aty = "" }},
		{"empty event_type", func(_, _, _, ety *string, _ *time.Time, _, _ *int64, _ *event.EventMetadata) { *ety = "" }},
		{"empty tenant_id", func(_, _, _, _ *string, _ *time.Time, _, _ *int64, m *event.EventMetadata) { m.TenantID = "" }},
		{"zero time", func(_, _, _, _ *string, tm *time.Time, _, _ *int64, _ *event.EventMetadata) { *tm = time.Time{} }},
		{"negative version", func(_, _, _, _ *string, _ *time.Time, v, _ *int64, _ *event.EventMetadata) { *v = -1 }},
		{"negative sequence", func(_, _, _, _ *string, _ *time.Time, _, s *int64, _ *event.EventMetadata) { *s = -1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			eid, aid, aty, ety, tm, v, s, m := eventID, aggregateID, aggregateType, eventType, at, version, seq, custom
			tc.mutate(&eid, &aid, &aty, &ety, &tm, &v, &s, &m)
			if _, err := event.NewBaseEvent(eid, aid, aty, ety, tm, v, s, nil, m); err == nil {
				t.Errorf("expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestBaseEventImmutability(t *testing.T) {
	t.Parallel()
	eventID, aggregateID, aggregateType, eventType, at, version, seq, _ := validArgs()
	input := event.EventMetadata{TenantID: testTenantID, Custom: map[string]string{"k": "v"}}

	evt, err := event.NewBaseEvent(eventID, aggregateID, aggregateType, eventType, at, version, seq, nil, input)
	if err != nil {
		t.Fatalf("NewBaseEvent returned error: %v", err)
	}
	// Mutating the caller's map must not affect the envelope.
	input.Custom["k"] = "mutated"
	input.Custom["evil"] = "injected"
	got := evt.Metadata()
	if got.Custom["k"] != "v" || len(got.Custom) != 1 {
		t.Fatalf("construction copy failed: %v", got.Custom)
	}
	// Mutating a returned Metadata copy must not affect the envelope.
	got.Custom["k"] = "mutated-again"
	again := evt.Metadata()
	if again.Custom["k"] != "v" || len(again.Custom) != 1 {
		t.Fatalf("accessor copy failed: %v", again.Custom)
	}
}

func TestBaseEventOccurredAtNormalizedUTC(t *testing.T) {
	t.Parallel()
	eventID, aggregateID, aggregateType, eventType, _, version, seq, meta := validArgs()
	eastern := time.FixedZone("EST", -5*60*60)
	local := time.Date(2026, 9, 14, 10, 0, 0, 0, eastern)

	evt, err := event.NewBaseEvent(eventID, aggregateID, aggregateType, eventType, local, version, seq, nil, meta)
	if err != nil {
		t.Fatalf("NewBaseEvent returned error: %v", err)
	}
	if evt.OccurredAt().Location() != time.UTC {
		t.Errorf("OccurredAt location = %v, want UTC", evt.OccurredAt().Location())
	}
	if !evt.OccurredAt().Equal(local) {
		t.Errorf("OccurredAt = %v, want instant %v", evt.OccurredAt(), local)
	}
}
