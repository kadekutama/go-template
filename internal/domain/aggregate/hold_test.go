package aggregate_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/aggregate"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func openTestHold(t *testing.T, id valueobject.HoldID, expires time.Time) *aggregate.Hold {
	t.Helper()
	at := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	h, err := aggregate.OpenHold(aggregate.OpenHoldParams{
		ID: id, TenantID: testTenantID, LedgerID: testLedgerID, AccountID: testAccount1,
		AssetCode: testUSD, AmountMinor: 100, Kind: "AUTHORIZATION",
		ExpiresAt: expires, CreatedAt: at,
	})
	if err != nil {
		t.Fatalf("OpenHold: %v", err)
	}
	return &h
}

func TestHoldCaptureIdempotent(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	h := openTestHold(t, "h-1", base.Add(time.Hour))
	if err := h.Capture(base.Add(time.Minute)); err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if h.State() != entity.HoldCaptured || h.Record().Version != 2 {
		t.Fatalf("state = %s version = %d", h.State(), h.Record().Version)
	}
	if err := h.Capture(base.Add(2 * time.Minute)); err != nil {
		t.Fatalf("repeat Capture must be idempotent: %v", err)
	}
	if h.Record().Version != 2 {
		t.Fatal("idempotent repeat must not bump version")
	}
	if err := h.Release(base.Add(3 * time.Minute)); err == nil {
		t.Fatal("Release after capture must fail")
	}
	if err := h.Expire(base.Add(2 * time.Hour)); err == nil {
		t.Fatal("Expire after capture must fail")
	}
}

func TestHoldReleaseAndExpire(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	h := openTestHold(t, "h-2", base.Add(time.Hour))
	if err := h.Release(base.Add(time.Minute)); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if err := h.Release(base.Add(2 * time.Minute)); err != nil {
		t.Fatalf("repeat Release must be idempotent: %v", err)
	}
	if err := h.Capture(base.Add(3 * time.Minute)); err == nil {
		t.Fatal("Capture after release must fail")
	}

	e := openTestHold(t, "h-3", base.Add(time.Hour))
	if err := e.Expire(base); err == nil || !strings.Contains(err.Error(), "HOLD_NOT_EXPIRED") {
		t.Fatalf("early Expire err = %v", err)
	}
	if err := e.Expire(base.Add(2 * time.Hour)); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if e.State() != entity.HoldExpired {
		t.Fatalf("state = %s", e.State())
	}
	if err := e.Expire(base.Add(3 * time.Hour)); err != nil {
		t.Fatalf("repeat Expire must be idempotent: %v", err)
	}
	if err := e.Capture(base.Add(3 * time.Hour)); err == nil || !strings.Contains(err.Error(), "HOLD_STATE_CONFLICT") {
		t.Fatalf("capture expired err = %v", err)
	}
}

func TestHoldLateCaptureExpired(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)

	type testCase struct {
		name          string
		captureAt     time.Time
		expectedError string
	}

	testCases := []testCase{
		{
			name:          "capture after expiration fails with HOLD_EXPIRED",
			captureAt:     base.Add(2 * time.Hour),
			expectedError: "HOLD_EXPIRED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := openTestHold(t, "h-4", base.Add(time.Hour))
			err := h.Capture(tc.captureAt)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestHoldValidation(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	valid := aggregate.OpenHoldParams{
		ID:          "h-9",
		TenantID:    testTenantID,
		LedgerID:    testLedgerID,
		AccountID:   testAccount1,
		AssetCode:   testUSD,
		AmountMinor: 100,
		Kind:        "AUTHORIZATION",
		ExpiresAt:   base.Add(time.Hour),
		CreatedAt:   base,
	}

	type testCase struct {
		name          string
		params        aggregate.OpenHoldParams
		expectedError bool
	}

	testCases := []testCase{
		{
			name:          "valid hold params pass",
			params:        valid,
			expectedError: false,
		},
		{
			name: "zero amount rejected",
			params: func() aggregate.OpenHoldParams {
				p := valid
				p.AmountMinor = 0
				return p
			}(),
			expectedError: true,
		},
		{
			name: "empty kind rejected",
			params: func() aggregate.OpenHoldParams {
				p := valid
				p.Kind = ""
				return p
			}(),
			expectedError: true,
		},
		{
			name: "empty id rejected",
			params: func() aggregate.OpenHoldParams {
				p := valid
				p.ID = ""
				return p
			}(),
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := aggregate.OpenHold(tc.params)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
