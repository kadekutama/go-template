package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
)

func TestNewTenantUpdated(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	meta := event.EventMetadata{TenantID: "t-1", LedgerID: "l-1"}
	basePayload := event.TenantUpdatedPayload{
		TenantID:  "t-1",
		Name:      "Acme",
		Region:    "us-east-1",
		Settings:  entity.TenantSettings{DefaultCurrency: "USD", Timezone: "UTC"},
		UpdatedBy: "u-1",
	}

	type testCase struct {
		name          string
		eventID       string
		aggregateID   string
		occurredAt    time.Time
		version       int64
		seq           int64
		payload       event.TenantUpdatedPayload
		meta          event.EventMetadata
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid updated event",
			eventID:       "ev-1",
			aggregateID:   "t-1",
			occurredAt:    at,
			version:       2,
			seq:           1,
			payload:       basePayload,
			meta:          meta,
			expectedError: nil,
		},
		{
			name:        "missing tenant id in payload",
			eventID:     "ev-1",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     2,
			seq:         1,
			payload: func() event.TenantUpdatedPayload {
				p := basePayload
				p.TenantID = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: tenant_id is required"),
		},
		{
			name:          "empty event id",
			eventID:       "",
			aggregateID:   "t-1",
			occurredAt:    at,
			version:       2,
			seq:           1,
			payload:       basePayload,
			meta:          meta,
			expectedError: errors.New("event: event_id is required"),
		},
		{
			name:          "zero occurred at",
			eventID:       "ev-1",
			aggregateID:   "t-1",
			occurredAt:    time.Time{},
			version:       2,
			seq:           1,
			payload:       basePayload,
			meta:          meta,
			expectedError: errors.New("event: occurred_at is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewTenantUpdated(tc.eventID, tc.aggregateID, tc.occurredAt, tc.version, tc.seq, tc.payload, tc.meta)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, "tenant.updated.v1", evt.EventType())
				assert.Equal(t, tc.aggregateID, evt.AggregateID())
				assert.Equal(t, tc.payload, evt.Payload())
			}
		})
	}
}

func TestNewTenantSuspended(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	meta := event.EventMetadata{TenantID: "t-1", LedgerID: "l-1"}
	basePayload := event.TenantSuspendedPayload{
		TenantID:    "t-1",
		Reason:      "compliance review",
		SuspendedBy: "u-1",
	}

	type testCase struct {
		name          string
		eventID       string
		aggregateID   string
		occurredAt    time.Time
		version       int64
		seq           int64
		payload       event.TenantSuspendedPayload
		meta          event.EventMetadata
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid suspended event",
			eventID:       "ev-2",
			aggregateID:   "t-1",
			occurredAt:    at,
			version:       3,
			seq:           2,
			payload:       basePayload,
			meta:          meta,
			expectedError: nil,
		},
		{
			name:        "missing tenant id in payload",
			eventID:     "ev-2",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     3,
			seq:         2,
			payload: func() event.TenantSuspendedPayload {
				p := basePayload
				p.TenantID = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: tenant_id is required"),
		},
		{
			name:        "empty reason",
			eventID:     "ev-2",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     3,
			seq:         2,
			payload: func() event.TenantSuspendedPayload {
				p := basePayload
				p.Reason = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: reason is required"),
		},
		{
			name:        "whitespace reason",
			eventID:     "ev-2",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     3,
			seq:         2,
			payload: func() event.TenantSuspendedPayload {
				p := basePayload
				p.Reason = "   "
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: reason is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewTenantSuspended(tc.eventID, tc.aggregateID, tc.occurredAt, tc.version, tc.seq, tc.payload, tc.meta)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, "tenant.suspended.v1", evt.EventType())
				assert.Equal(t, tc.aggregateID, evt.AggregateID())
				assert.Equal(t, tc.payload, evt.Payload())
			}
		})
	}
}

func TestNewTenantReactivated(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	meta := event.EventMetadata{TenantID: "t-1", LedgerID: "l-1"}
	basePayload := event.TenantReactivatedPayload{
		TenantID:      "t-1",
		ReactivatedBy: "u-1",
	}

	type testCase struct {
		name          string
		eventID       string
		aggregateID   string
		occurredAt    time.Time
		version       int64
		seq           int64
		payload       event.TenantReactivatedPayload
		meta          event.EventMetadata
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid reactivated event",
			eventID:       "ev-3",
			aggregateID:   "t-1",
			occurredAt:    at,
			version:       4,
			seq:           3,
			payload:       basePayload,
			meta:          meta,
			expectedError: nil,
		},
		{
			name:        "missing tenant id in payload",
			eventID:     "ev-3",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     4,
			seq:         3,
			payload: func() event.TenantReactivatedPayload {
				p := basePayload
				p.TenantID = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: tenant_id is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewTenantReactivated(tc.eventID, tc.aggregateID, tc.occurredAt, tc.version, tc.seq, tc.payload, tc.meta)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, "tenant.reactivated.v1", evt.EventType())
				assert.Equal(t, tc.aggregateID, evt.AggregateID())
				assert.Equal(t, tc.payload, evt.Payload())
			}
		})
	}
}

func TestNewTenantClosed(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	meta := event.EventMetadata{TenantID: "t-1", LedgerID: "l-1"}
	basePayload := event.TenantClosedPayload{
		TenantID: "t-1",
		Reason:   "voluntary closure",
		ClosedBy: "u-1",
	}

	type testCase struct {
		name          string
		eventID       string
		aggregateID   string
		occurredAt    time.Time
		version       int64
		seq           int64
		payload       event.TenantClosedPayload
		meta          event.EventMetadata
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid closed event",
			eventID:       "ev-4",
			aggregateID:   "t-1",
			occurredAt:    at,
			version:       5,
			seq:           4,
			payload:       basePayload,
			meta:          meta,
			expectedError: nil,
		},
		{
			name:        "missing tenant id in payload",
			eventID:     "ev-4",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     5,
			seq:         4,
			payload: func() event.TenantClosedPayload {
				p := basePayload
				p.TenantID = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: tenant_id is required"),
		},
		{
			name:        "empty reason",
			eventID:     "ev-4",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     5,
			seq:         4,
			payload: func() event.TenantClosedPayload {
				p := basePayload
				p.Reason = ""
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: reason is required"),
		},
		{
			name:        "whitespace reason",
			eventID:     "ev-4",
			aggregateID: "t-1",
			occurredAt:  at,
			version:     5,
			seq:         4,
			payload: func() event.TenantClosedPayload {
				p := basePayload
				p.Reason = "   "
				return p
			}(),
			meta:          meta,
			expectedError: errors.New("event: reason is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evt, err := event.NewTenantClosed(tc.eventID, tc.aggregateID, tc.occurredAt, tc.version, tc.seq, tc.payload, tc.meta)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, "tenant.closed.v1", evt.EventType())
				assert.Equal(t, tc.aggregateID, evt.AggregateID())
				assert.Equal(t, tc.payload, evt.Payload())
			}
		})
	}
}
