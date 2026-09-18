package aggregate_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func openUnassignedTestTenant(t *testing.T) aggregate.Tenant {
	t.Helper()
	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)
	tn, err := aggregate.OpenTenant(aggregate.OpenTenantParams{
		ID:       "",
		LedgerID: testLedgerID,
		Name:     "Acme Corp",
		Region:   "us-east-1",
		Settings: entity.TenantSettings{
			DefaultCurrency: "USD",
			Timezone:        "UTC",
		},
		OpenedBy:   testUser1,
		EventID:    testEvent0,
		OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenTenant unassigned: %v", err)
	}
	return tn
}

func openTestTenant(t *testing.T) *aggregate.Tenant {
	t.Helper()
	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)
	tn, err := aggregate.OpenTenant(aggregate.OpenTenantParams{
		ID:       testTenantID,
		LedgerID: testLedgerID,
		Name:     "Acme Corp",
		Region:   "us-east-1",
		Settings: entity.TenantSettings{
			DefaultCurrency: "USD",
			Timezone:        "UTC",
		},
		OpenedBy:   testUser1,
		EventID:    "ev-0",
		OccurredAt: at,
	})
	if err != nil {
		t.Fatalf("OpenTenant: %v", err)
	}
	return &tn
}

func TestOpenTenant(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		p             aggregate.OpenTenantParams
		expectedError error
	}

	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)
	baseParams := aggregate.OpenTenantParams{
		ID:       testTenantID,
		LedgerID: testLedgerID,
		Name:     "Acme Corp",
		Region:   "us-east-1",
		Settings: entity.TenantSettings{
			DefaultCurrency: "USD",
			Timezone:        "UTC",
		},
		OpenedBy:   testUser1,
		EventID:    "ev-0",
		OccurredAt: at,
	}

	testCases := []testCase{
		{
			name:          "valid open",
			p:             baseParams,
			expectedError: nil,
		},
		{
			name: "missing tenant id permitted before persistence",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.ID = ""
				return p
			}(),
			expectedError: nil,
		},
		{
			name: "invalid tenant id format",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.ID = "not-a-uuid"
				return p
			}(),
			expectedError: entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "missing ledger permitted before persistence",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.LedgerID = ""
				return p
			}(),
			expectedError: nil,
		},
		{
			name: "invalid ledger id format",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.LedgerID = "not-a-uuid"
				return p
			}(),
			expectedError: entity.NewError("LEDGER_ID_INVALID", "ledger id is invalid"),
		},
		{
			name: "missing opened by",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.OpenedBy = ""
				return p
			}(),
			expectedError: entity.NewError("OPENED_BY_REQUIRED", "opened by user id is required"),
		},
		{
			name: "empty event id",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.EventID = ""
				return p
			}(),
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name: "whitespace event id",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.EventID = "   "
				return p
			}(),
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name: "zero occurred at",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.OccurredAt = time.Time{}
				return p
			}(),
			expectedError: entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
		{
			name: "whitespace name rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.Name = "  "
				return p
			}(),
			expectedError: entity.NewError("TENANT_NAME_REQUIRED", "tenant name is required"),
		},
		{
			name: "invalid settings rejected",
			p: func() aggregate.OpenTenantParams {
				p := baseParams
				p.Settings.Timezone = ""
				return p
			}(),
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tn, err := aggregate.OpenTenant(tc.p)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, entity.TenantActive, tn.Record().Status)
				assert.Equal(t, int64(1), tn.Record().Version)
				evts := tn.UncommittedEvents()
				if tc.p.ID.String() != "" && tc.p.LedgerID.String() != "" {
					require.Len(t, evts, 1)
					assert.Equal(t, "tenant.created.v1", evts[0].EventType())
				} else {
					assert.Empty(t, evts)
				}
			}
		})
	}
}

func TestTenantLifecycleEmissionOrder(t *testing.T) {
	t.Parallel()

	tn := openTestTenant(t)
	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	ledger := testLedgerID
	tr := func(ev string) aggregate.TenantTransitionParams {
		return aggregate.TenantTransitionParams{Actor: testUser1, EventID: ev, OccurredAt: at}
	}
	require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{TenantTransitionParams: tr("ev-1"), Reason: "review"}, ledger))
	require.NoError(t, tn.Reactivate(tr("ev-2"), ledger))
	require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{TenantTransitionParams: tr("ev-3"), Reason: "again"}, ledger))
	require.NoError(t, tn.Close(aggregate.TenantCloseParams{TenantTransitionParams: tr("ev-4"), Reason: "obsolete"}, ledger))
	assert.Equal(t, int64(5), tn.Record().Version)
	want := []string{"tenant.created.v1", "tenant.suspended.v1", "tenant.reactivated.v1", "tenant.suspended.v1", "tenant.closed.v1"}
	evts := tn.UncommittedEvents()
	require.Len(t, evts, len(want))
	for i, w := range want {
		assert.Equal(t, w, evts[i].EventType())
		assert.Equal(t, testTenantID.String(), evts[i].AggregateID())
	}
	tn.ClearEvents()
	assert.Empty(t, tn.UncommittedEvents())
}

func TestTenantSuspend(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	baseTransition := aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-1", OccurredAt: at}
	baseParams := aggregate.TenantSuspendParams{TenantTransitionParams: baseTransition, Reason: "compliance review"}
	baseLedger := testLedgerID

	type testCase struct {
		name          string
		initialState  string
		p             aggregate.TenantSuspendParams
		ledgerID      valueobject.LedgerID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "suspend active tenant succeeds",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "suspend already suspended tenant rejected",
			initialState:  "suspended",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_ALREADY_SUSPENDED", "tenant is already suspended"),
		},
		{
			name:          "suspend closed tenant rejected",
			initialState:  "closed",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_CLOSED", "tenant is closed"),
		},
		{
			name:         "empty reason rejected",
			initialState: "active",
			p: func() aggregate.TenantSuspendParams {
				p := baseParams
				p.Reason = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_REASON_REQUIRED", "suspend reason is required"),
		},
		{
			name:         "whitespace reason rejected",
			initialState: "active",
			p: func() aggregate.TenantSuspendParams {
				p := baseParams
				p.Reason = "   "
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_REASON_REQUIRED", "suspend reason is required"),
		},
		{
			name:          "missing ledger rejected",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      "",
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name:         "empty event id rejected",
			initialState: "active",
			p: func() aggregate.TenantSuspendParams {
				p := baseParams
				p.EventID = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name:         "zero occurred at rejected",
			initialState: "active",
			p: func() aggregate.TenantSuspendParams {
				p := baseParams
				p.OccurredAt = time.Time{}
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tn := openTestTenant(t)
			switch tc.initialState {
			case "suspended":
				require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			case "closed":
				require.NoError(t, tn.Close(aggregate.TenantCloseParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			}

			err := tn.Suspend(tc.p, tc.ledgerID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, entity.TenantSuspended, tn.Record().Status)
				assert.Equal(t, int64(2), tn.Record().Version)
				evts := tn.UncommittedEvents()
				require.Len(t, evts, 2)
				assert.Equal(t, "tenant.suspended.v1", evts[1].EventType())
			}
		})
	}
}

func TestTenantReactivate(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	baseTransition := aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-reactivate", OccurredAt: at}
	baseLedger := testLedgerID

	type testCase struct {
		name          string
		initialState  string
		p             aggregate.TenantTransitionParams
		ledgerID      valueobject.LedgerID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "reactivate suspended tenant succeeds",
			initialState:  "suspended",
			p:             baseTransition,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "reactivate active tenant rejected",
			initialState:  "active",
			p:             baseTransition,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_NOT_SUSPENDED", "tenant is not suspended"),
		},
		{
			name:          "reactivate closed tenant rejected",
			initialState:  "closed",
			p:             baseTransition,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_CLOSED", "tenant is closed"),
		},
		{
			name:          "missing ledger rejected",
			initialState:  "suspended",
			p:             baseTransition,
			ledgerID:      "",
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name:         "empty event id rejected",
			initialState: "suspended",
			p: func() aggregate.TenantTransitionParams {
				p := baseTransition
				p.EventID = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name:         "zero occurred at rejected",
			initialState: "suspended",
			p: func() aggregate.TenantTransitionParams {
				p := baseTransition
				p.OccurredAt = time.Time{}
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tn := openTestTenant(t)
			switch tc.initialState {
			case "suspended":
				require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			case "closed":
				require.NoError(t, tn.Close(aggregate.TenantCloseParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			}

			err := tn.Reactivate(tc.p, tc.ledgerID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, entity.TenantActive, tn.Record().Status)
				assert.Equal(t, int64(3), tn.Record().Version)
				evts := tn.UncommittedEvents()
				require.Len(t, evts, 3)
				assert.Equal(t, "tenant.reactivated.v1", evts[2].EventType())
			}
		})
	}
}

func TestTenantClose(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	baseTransition := aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-close", OccurredAt: at}
	baseParams := aggregate.TenantCloseParams{TenantTransitionParams: baseTransition, Reason: "account closure"}
	baseLedger := testLedgerID

	type testCase struct {
		name          string
		initialState  string
		p             aggregate.TenantCloseParams
		ledgerID      valueobject.LedgerID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "close active tenant succeeds",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "close suspended tenant succeeds",
			initialState:  "suspended",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "close already closed tenant rejected",
			initialState:  "closed",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_CLOSED", "tenant is closed"),
		},
		{
			name:         "empty reason rejected",
			initialState: "active",
			p: func() aggregate.TenantCloseParams {
				p := baseParams
				p.Reason = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_REASON_REQUIRED", "close reason is required"),
		},
		{
			name:         "whitespace reason rejected",
			initialState: "active",
			p: func() aggregate.TenantCloseParams {
				p := baseParams
				p.Reason = "   "
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_REASON_REQUIRED", "close reason is required"),
		},
		{
			name:          "missing ledger rejected",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      "",
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name:         "empty event id rejected",
			initialState: "active",
			p: func() aggregate.TenantCloseParams {
				p := baseParams
				p.EventID = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name:         "zero occurred at rejected",
			initialState: "active",
			p: func() aggregate.TenantCloseParams {
				p := baseParams
				p.OccurredAt = time.Time{}
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tn := openTestTenant(t)
			switch tc.initialState {
			case "suspended":
				require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			case "closed":
				require.NoError(t, tn.Close(aggregate.TenantCloseParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "setup",
				}, baseLedger))
			}

			err := tn.Close(tc.p, tc.ledgerID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, entity.TenantClosed, tn.Record().Status)
				evts := tn.UncommittedEvents()
				require.NotEmpty(t, evts)
				assert.Equal(t, "tenant.closed.v1", evts[len(evts)-1].EventType())
			}
		})
	}
}

func TestTenantUpdateSettings(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 6, 0, 0, 0, time.UTC)
	baseTransition := aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-update", OccurredAt: at}
	validSettings := entity.TenantSettings{
		DefaultCurrency:       "EUR",
		Timezone:              "Europe/Berlin",
		EnabledFeatures:       []string{"transfers"},
		EnabledPaymentMethods: []string{"ach"},
	}
	baseParams := aggregate.TenantUpdateSettingsParams{TenantTransitionParams: baseTransition, Settings: validSettings}
	baseLedger := testLedgerID

	type testCase struct {
		name          string
		initialState  string
		p             aggregate.TenantUpdateSettingsParams
		ledgerID      valueobject.LedgerID
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid update on active tenant succeeds",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "valid update on suspended tenant succeeds",
			initialState:  "suspended",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: nil,
		},
		{
			name:          "update on closed tenant rejected",
			initialState:  "closed",
			p:             baseParams,
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_CLOSED", "tenant is closed"),
		},
		{
			name:         "invalid settings rejected",
			initialState: "active",
			p: func() aggregate.TenantUpdateSettingsParams {
				p := baseParams
				p.Settings.Timezone = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("TENANT_SETTINGS_INVALID", "timezone is required"),
		},
		{
			name:          "missing ledger rejected",
			initialState:  "active",
			p:             baseParams,
			ledgerID:      "",
			expectedError: entity.NewError("LEDGER_REQUIRED", "ledger id is required"),
		},
		{
			name:         "empty event id rejected",
			initialState: "active",
			p: func() aggregate.TenantUpdateSettingsParams {
				p := baseParams
				p.EventID = ""
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("EVENT_ID_REQUIRED", "event id is required"),
		},
		{
			name:         "zero occurred at rejected",
			initialState: "active",
			p: func() aggregate.TenantUpdateSettingsParams {
				p := baseParams
				p.OccurredAt = time.Time{}
				return p
			}(),
			ledgerID:      baseLedger,
			expectedError: entity.NewError("OCCURRED_AT_REQUIRED", "occurred at time is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tn := openTestTenant(t)
			switch tc.initialState {
			case "suspended":
				require.NoError(t, tn.Suspend(aggregate.TenantSuspendParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "review",
				}, baseLedger))
			case "closed":
				require.NoError(t, tn.Close(aggregate.TenantCloseParams{
					TenantTransitionParams: aggregate.TenantTransitionParams{Actor: testUser1, EventID: "ev-pre", OccurredAt: at},
					Reason:                 "done",
				}, baseLedger))
			}
			beforeVersion := tn.Record().Version
			err := tn.UpdateSettings(tc.p, tc.ledgerID)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.p.Settings, tn.Record().Settings)
				assert.Equal(t, beforeVersion+1, tn.Record().Version)
				evts := tn.UncommittedEvents()
				require.NotEmpty(t, evts)
				assert.Equal(t, "tenant.updated.v1", evts[len(evts)-1].EventType())
			}
		})
	}
}

func TestTenantMutationAtomicity(t *testing.T) {
	t.Parallel()

	tn := openTestTenant(t)
	initialRecord := tn.Record()
	initialEvents := tn.UncommittedEvents()

	// Invalid Suspend call (empty reason) must leave state completely unmutated
	err := tn.Suspend(aggregate.TenantSuspendParams{
		TenantTransitionParams: aggregate.TenantTransitionParams{
			Actor:      testUser1,
			EventID:    "ev-fail",
			OccurredAt: time.Now().UTC(),
		},
		Reason: "   ",
	}, testLedgerID)
	assert.Error(t, err)
	assert.Equal(t, initialRecord.Version, tn.Record().Version)
	assert.Equal(t, initialRecord.Status, tn.Record().Status)
	assert.Equal(t, len(initialEvents), len(tn.UncommittedEvents()))

	// Invalid UpdateSettings call (missing timezone) must leave state completely unmutated
	err = tn.UpdateSettings(aggregate.TenantUpdateSettingsParams{
		TenantTransitionParams: aggregate.TenantTransitionParams{
			Actor:      testUser1,
			EventID:    "ev-fail2",
			OccurredAt: time.Now().UTC(),
		},
		Settings: entity.TenantSettings{
			DefaultCurrency: "EUR",
			Timezone:        "",
		},
	}, testLedgerID)
	assert.Error(t, err)
	assert.Equal(t, initialRecord.Version, tn.Record().Version)
	assert.Equal(t, initialRecord.Settings, tn.Record().Settings)
	assert.Equal(t, len(initialEvents), len(tn.UncommittedEvents()))
}

func TestTenantAssignID(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 9, 14, 5, 0, 0, 0, time.UTC)

	type testCase struct {
		name          string
		tn            aggregate.Tenant
		id            valueobject.TenantID
		ledgerID      valueobject.LedgerID
		eventID       string
		occurredAt    time.Time
		openedBy      valueobject.UserID
		expectedError error
	}

	testCases := []testCase{
		{
			name: "successful assignment to unassigned tenant",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            testTenantID,
			ledgerID:      testLedgerID,
			eventID:       testEvent1,
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: nil,
		},
		{
			name: "double assignment rejected with immutable error",
			tn: func() aggregate.Tenant {
				tn := openUnassignedTestTenant(t)
				err := tn.AssignID(testTenantID, testLedgerID, testEvent1, at, testUser1)
				require.NoError(t, err)
				return tn
			}(),
			id:            testTenantID,
			ledgerID:      testLedgerID,
			eventID:       testEvent2,
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: entity.NewError("TENANT_ID_IMMUTABLE", "tenant id is already assigned"),
		},
		{
			name: "malformed tenant id rejected",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            "not-a-valid-uuid",
			ledgerID:      testLedgerID,
			eventID:       testEvent1,
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "empty tenant id rejected",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            "",
			ledgerID:      testLedgerID,
			eventID:       testEvent1,
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: entity.NewError("TENANT_ID_INVALID", "tenant id is invalid"),
		},
		{
			name: "empty ledger id rejected",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            testTenantID,
			ledgerID:      "",
			eventID:       testEvent1,
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: errors.New("event: ledger_id is required"),
		},
		{
			name: "empty event id rejected",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            testTenantID,
			ledgerID:      testLedgerID,
			eventID:       "",
			occurredAt:    at,
			openedBy:      testUser1,
			expectedError: errors.New("event: event_id is required"),
		},
		{
			name: "zero occurred at rejected",
			tn: func() aggregate.Tenant {
				return openUnassignedTestTenant(t)
			}(),
			id:            testTenantID,
			ledgerID:      testLedgerID,
			eventID:       testEvent1,
			occurredAt:    time.Time{},
			openedBy:      testUser1,
			expectedError: errors.New("event: occurred_at is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.tn.AssignID(tc.id, tc.ledgerID, tc.eventID, tc.occurredAt, tc.openedBy)
			assert.Equal(t, tc.expectedError, err)
			if tc.expectedError == nil {
				assert.Equal(t, tc.id, tc.tn.Record().ID)
				evts := tc.tn.UncommittedEvents()
				require.Len(t, evts, 1)
				assert.Equal(t, "tenant.created.v1", evts[0].EventType())
				assert.Equal(t, tc.id.String(), evts[0].AggregateID())
			}
		})
	}
}
