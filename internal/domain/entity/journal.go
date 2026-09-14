package entity

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Journal groups immutable postings with metadata. All postings in a journal
// share the journal tenant, ledger, and period.
type Journal struct {
	ID          valueobject.JournalID
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	PeriodID    valueobject.PeriodID
	PostingIDs  []valueobject.PostingID
	Description string
	Metadata    map[string]string
	CreatedAt   time.Time
}

// NewJournal validates uniform scope across the journal and its postings.
// Mixed tenants or ledgers fail; number/period placement checks that need the
// period aggregate live with the caller.
func NewJournal(id valueobject.JournalID, tenant valueobject.TenantID, ledger valueobject.LedgerID, period valueobject.PeriodID, postings []PostingData, description string, metadata map[string]string, createdAt time.Time) (Journal, error) {
	if id.String() == "" {
		return Journal{}, NewError("JOURNAL_ID_REQUIRED", "journal id is required")
	}
	if tenant.String() == "" {
		return Journal{}, NewError("TENANT_REQUIRED", "tenant id is required")
	}
	if ledger.String() == "" {
		return Journal{}, NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if period.String() == "" {
		return Journal{}, NewError("JOURNAL_PERIOD_REQUIRED", "period id is required")
	}
	if len(postings) == 0 {
		return Journal{}, NewError("JOURNAL_POSTINGS_REQUIRED", "journal requires at least one posting")
	}
	if createdAt.IsZero() {
		return Journal{}, NewError("JOURNAL_TIME_REQUIRED", "creation time is required")
	}
	ids := make([]valueobject.PostingID, 0, len(postings))
	for _, p := range postings {
		if p.TenantID != tenant || p.LedgerID != ledger {
			return Journal{}, NewError("JOURNAL_SCOPE_MISMATCH", "all postings must share the journal tenant and ledger")
		}
		ids = append(ids, p.ID)
	}
	md := map[string]string(nil)
	if metadata != nil {
		md = make(map[string]string, len(metadata))
		for k, v := range metadata {
			md[k] = v
		}
	}
	return Journal{ID: id, TenantID: tenant, LedgerID: ledger, PeriodID: period,
		PostingIDs: ids, Description: description, Metadata: md, CreatedAt: createdAt.UTC()}, nil
}
