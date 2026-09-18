package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PostingData is the persistence record for an immutable accounting fact.
// An accepted posting has no pending/failed/voided state; corrections are new
// linked postings.
type PostingData struct {
	ID                valueobject.PostingID
	TenantID          valueobject.TenantID
	LedgerID          valueobject.LedgerID
	Operation         string
	ExternalReference string
	Description       string
	Entries           []Entry
	EffectiveAt       time.Time
	RecordedAt        time.Time
	ReversalOf        *valueobject.PostingID
	Reason            string
	Metadata          map[string]string
}

func (p PostingData) validateHeader() error {
	if p.ID.String() != "" {
		if _, err := valueobject.ParsePostingID(p.ID.String()); err != nil {
			return NewError("POSTING_ID_INVALID", "posting id is invalid")
		}
	}
	if p.TenantID.String() == "" {
		return NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if p.LedgerID.String() == "" {
		return NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if p.Operation == "" {
		return NewError("POSTING_OPERATION_REQUIRED", "operation template name is required")
	}
	return nil
}

func (p PostingData) validateEntries() error {
	if len(p.Entries) < 2 {
		return NewError("POSTING_ENTRIES_REQUIRED", "posting requires at least two entries")
	}
	for i := range p.Entries {
		if err := p.Entries[i].Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Validate checks posting structure (entry count, per-entry validity, required
// scope and timestamps). An empty ID is permitted prior to persistence;
// if set, it must be a valid canonical UUID.
func (p PostingData) Validate() error {
	if err := p.validateHeader(); err != nil {
		return err
	}
	if err := p.validateEntries(); err != nil {
		return err
	}
	if p.EffectiveAt.IsZero() {
		return NewError("POSTING_EFFECTIVE_REQUIRED", "effective_at is required")
	}
	if p.RecordedAt.IsZero() {
		return NewError("POSTING_RECORDED_REQUIRED", "recorded_at is server-assigned and required")
	}
	if p.ReversalOf != nil && p.Reason == "" {
		return NewError("REVERSAL_REASON_REQUIRED", "reversal requires a reason")
	}
	return nil
}
