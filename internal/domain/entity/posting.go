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

// Validate checks posting structure (entry count, per-entry validity, required
// scope and timestamps). Per-asset balance and account checks are construction
// rules in the aggregate, where the account set is available.
func (p PostingData) Validate() error {
	if p.ID.String() == "" {
		return NewError("POSTING_ID_REQUIRED", "posting id is required")
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
	if len(p.Entries) < 2 {
		return NewError("POSTING_ENTRIES_REQUIRED", "posting requires at least two entries")
	}
	for i := range p.Entries {
		if err := p.Entries[i].Validate(); err != nil {
			return err
		}
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
