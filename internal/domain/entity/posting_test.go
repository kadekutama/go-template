package entity_test

import (
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func validEntry() entity.Entry {
	return entity.Entry{ID: "e-1", PostingID: testPosting1, AccountID: testAccount1, Side: valueobject.DirectionDebit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1}
}

func TestEntryValidate(t *testing.T) {
	t.Parallel()
	if err := validEntry().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*entity.Entry)
		code   string
	}{
		{testEmptyID, func(e *entity.Entry) { e.ID = "" }, "ENTRY_ID_REQUIRED"},
		{"empty posting", func(e *entity.Entry) { e.PostingID = "" }, "ENTRY_POSTING_REQUIRED"},
		{"empty account", func(e *entity.Entry) { e.AccountID = "" }, "ENTRY_ACCOUNT_REQUIRED"},
		{"bad side", func(e *entity.Entry) { e.Side = "SIDEWAYS" }, "ENTRY_SIDE_INVALID"},
		{"zero amount", func(e *entity.Entry) { e.AmountMinor = 0 }, "INVALID_ENTRY_AMOUNT"},
		{"negative amount", func(e *entity.Entry) { e.AmountMinor = -5 }, "INVALID_ENTRY_AMOUNT"},
		{testEmptyAsset, func(e *entity.Entry) { e.AssetCode = "" }, "ENTRY_ASSET_REQUIRED"},
		{"zero seq", func(e *entity.Entry) { e.AccountSeq = 0 }, "ENTRY_SEQUENCE_REQUIRED"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := validEntry()
			tc.mutate(&e)
			err, ok := e.Validate().(*entity.Error)
			if !ok || err == nil {
				t.Fatalf("err = %v, want coded %s", err, tc.code)
			}
			if err.Code != tc.code {
				t.Fatalf("code = %s, want %s", err.Code, tc.code)
			}
		})
	}
}

func validPostingData() entity.PostingData {
	at := time.Date(2026, 9, 14, 7, 0, 0, 0, time.UTC)
	return entity.PostingData{
		ID: testPosting1, TenantID: testTenantID, LedgerID: testLedgerID, Operation: "transfer.v1",
		Entries: []entity.Entry{
			validEntry(),
			{ID: "e-2", PostingID: testPosting1, AccountID: testAccount2, Side: valueobject.DirectionCredit, AmountMinor: 100, AssetCode: testUSD, AccountSeq: 1},
		},
		EffectiveAt: at, RecordedAt: at,
	}
}

func TestPostingDataValidate(t *testing.T) {
	t.Parallel()
	if err := validPostingData().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	p := validPostingData()
	p.ID = ""
	if err := p.Validate(); err == nil {
		t.Error("empty id must error")
	}
	p = validPostingData()
	p.TenantID = ""
	if err := p.Validate(); err == nil {
		t.Error("empty tenant must error")
	}
	p = validPostingData()
	p.LedgerID = ""
	if err := p.Validate(); err == nil {
		t.Error("empty ledger must error")
	}
	p = validPostingData()
	p.Operation = ""
	if err := p.Validate(); err == nil {
		t.Error("empty operation must error")
	}
	p = validPostingData()
	p.Entries = p.Entries[:1]
	if err := p.Validate(); err == nil {
		t.Error("single entry must error")
	}
	p = validPostingData()
	p.Entries[0].AmountMinor = 0
	if err := p.Validate(); err == nil {
		t.Error("bad entry must error")
	}
	p = validPostingData()
	p.EffectiveAt = time.Time{}
	if err := p.Validate(); err == nil {
		t.Error("zero effective must error")
	}
	p = validPostingData()
	p.RecordedAt = time.Time{}
	if err := p.Validate(); err == nil {
		t.Error("zero recorded must error")
	}
	p = validPostingData()
	rev := valueobject.PostingID("p-0")
	p.ReversalOf = &rev
	if err := p.Validate(); err == nil {
		t.Error("reversal without reason must error")
	}
	p.Reason = "fix"
	if err := p.Validate(); err != nil {
		t.Errorf("reversal with reason: %v", err)
	}
}

func validHoldData() entity.HoldData {
	base := time.Date(2026, 9, 14, 9, 0, 0, 0, time.UTC)
	return entity.HoldData{
		ID: "h-1", TenantID: testTenantID, LedgerID: testLedgerID, AccountID: testAccount1,
		AssetCode: testUSD, AmountMinor: 100, Kind: "AUTHORIZATION", State: entity.HoldActive,
		ExpiresAt: base.Add(time.Hour), Version: 1, CreatedAt: base, UpdatedAt: base,
	}
}

func TestHoldDataValidate(t *testing.T) {
	t.Parallel()
	if err := validHoldData().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*entity.HoldData)
	}{
		{testEmptyID, func(h *entity.HoldData) { h.ID = "" }},
		{"empty tenant", func(h *entity.HoldData) { h.TenantID = "" }},
		{"empty ledger", func(h *entity.HoldData) { h.LedgerID = "" }},
		{"empty account", func(h *entity.HoldData) { h.AccountID = "" }},
		{testEmptyAsset, func(h *entity.HoldData) { h.AssetCode = "" }},
		{"zero amount", func(h *entity.HoldData) { h.AmountMinor = 0 }},
		{"empty kind", func(h *entity.HoldData) { h.Kind = "" }},
		{"bad state", func(h *entity.HoldData) { h.State = "MELTED" }},
		{"zero expiry", func(h *entity.HoldData) { h.ExpiresAt = time.Time{} }},
		{"zero version", func(h *entity.HoldData) { h.Version = 0 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			h := validHoldData()
			tc.mutate(&h)
			if err := h.Validate(); err == nil {
				t.Error("must error")
			}
		})
	}
	for _, s := range []string{entity.HoldActive, entity.HoldCaptured, entity.HoldReleased, entity.HoldExpired} {
		h := validHoldData()
		h.State = s
		if err := h.Validate(); err != nil {
			t.Errorf("state %s: %v", s, err)
		}
	}
}
