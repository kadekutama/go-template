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

	at := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	baseMeta := event.EventMetadata{
		TenantID:      testTenantID,
		LedgerID:      testLedgerID,
		CausationID:   testCmdID,
		CorrelationID: testCorrID,
		UserID:        testUserID,
		TraceID:       testTraceID,
		Custom:        map[string]string{"k": "v"},
	}

	type testCase struct {
		name             string
		eventID          string
		aggregateID      string
		aggregateType    string
		eventType        string
		occurredAt       time.Time
		aggregateVersion int64
		sequence         int64
		payload          any
		meta             event.EventMetadata
		expectedError    error
	}

	testCases := []testCase{
		{
			name:             "valid base event with all accessors",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 3,
			sequence:         7,
			payload:          map[string]string{"field": "value"},
			meta:             baseMeta,
			expectedError:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewBaseEvent(
				tc.eventID,
				tc.aggregateID,
				tc.aggregateType,
				tc.eventType,
				tc.occurredAt,
				tc.aggregateVersion,
				tc.sequence,
				tc.payload,
				tc.meta,
			)
			if tc.expectedError != nil {
				if err == nil || err.Error() != tc.expectedError.Error() {
					t.Fatalf("expected error %v, got %v", tc.expectedError, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var _ event.DomainEvent = evt
			verifyBaseAccessors(t, evt, tc.eventID, tc.aggregateID, tc.aggregateType, tc.eventType, tc.occurredAt, tc.aggregateVersion, tc.sequence)
			verifyBaseMetadata(t, evt.Metadata())
			if payload, ok := evt.Payload().(map[string]string); !ok || payload["field"] != "value" {
				t.Errorf("Payload = %v, want map[field:value]", evt.Payload())
			}
		})
	}
}

func TestNewBaseEventValidationMatrix(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	baseMeta := event.EventMetadata{TenantID: testTenantID, Custom: map[string]string{"k": "v"}}

	type testCase struct {
		name             string
		eventID          string
		aggregateID      string
		aggregateType    string
		eventType        string
		occurredAt       time.Time
		aggregateVersion int64
		sequence         int64
		payload          any
		meta             event.EventMetadata
		expectedError    string
	}

	testCases := []testCase{
		{
			name:             "empty event_id",
			eventID:          "",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: event_id is required",
		},
		{
			name:             "empty aggregate_id",
			eventID:          "evt-01",
			aggregateID:      "",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: aggregate_id is required",
		},
		{
			name:             "empty aggregate_type",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: aggregate_type is required",
		},
		{
			name:             "empty event_type",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: event_type is required",
		},
		{
			name:             "empty tenant_id",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta: func() event.EventMetadata {
				m := baseMeta
				m.TenantID = ""
				return m
			}(),
			expectedError: "event: metadata tenant_id is required",
		},
		{
			name:             "zero time",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       time.Time{},
			aggregateVersion: 1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: occurred_at is required",
		},
		{
			name:             "negative version",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: -1,
			sequence:         0,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: aggregate_version must not be negative",
		},
		{
			name:             "negative sequence",
			eventID:          "evt-01",
			aggregateID:      "agg-01",
			aggregateType:    "Account",
			eventType:        "account.created.v1",
			occurredAt:       at,
			aggregateVersion: 1,
			sequence:         -1,
			payload:          nil,
			meta:             baseMeta,
			expectedError:    "event: sequence must not be negative",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := event.NewBaseEvent(
				tc.eventID,
				tc.aggregateID,
				tc.aggregateType,
				tc.eventType,
				tc.occurredAt,
				tc.aggregateVersion,
				tc.sequence,
				tc.payload,
				tc.meta,
			)
			if err == nil || err.Error() != tc.expectedError {
				t.Fatalf("expected error %q, got %v", tc.expectedError, err)
			}
		})
	}
}

func TestBaseEventImmutability(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	input := event.EventMetadata{TenantID: testTenantID, Custom: map[string]string{"k": "v"}}

	evt, err := event.NewBaseEvent("evt-01", "agg-01", "Account", "account.created.v1", at, 1, 0, nil, input)
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

	eastern := time.FixedZone("EST", -5*60*60)
	local := time.Date(2026, 9, 14, 10, 0, 0, 0, eastern)
	meta := event.EventMetadata{TenantID: testTenantID}

	type testCase struct {
		name       string
		occurredAt time.Time
	}

	testCases := []testCase{
		{
			name:       "eastern timezone converted to utc",
			occurredAt: local,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewBaseEvent("evt-01", "agg-01", "Account", "account.created.v1", tc.occurredAt, 1, 0, nil, meta)
			if err != nil {
				t.Fatalf("NewBaseEvent returned error: %v", err)
			}
			if evt.OccurredAt().Location() != time.UTC {
				t.Errorf("OccurredAt location = %v, want UTC", evt.OccurredAt().Location())
			}
			if !evt.OccurredAt().Equal(tc.occurredAt) {
				t.Errorf("OccurredAt = %v, want instant %v", evt.OccurredAt(), tc.occurredAt)
			}
		})
	}
}
