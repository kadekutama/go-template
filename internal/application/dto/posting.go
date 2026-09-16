package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// PostingDTO is one committed posting with its lines and read cursor.
type PostingDTO struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	LedgerID    string     `json:"ledger_id"`
	Operation   string     `json:"operation"`
	Reference   string     `json:"reference"`
	Description string     `json:"description"`
	Entries     []EntryDTO `json:"entries"`
	EffectiveAt time.Time  `json:"effective_at"`
	RecordedAt  time.Time  `json:"recorded_at"`
	Cursor      string     `json:"cursor"`
}

// ToPostingDTO maps one committed posting plus its read cursor to the edge.
func ToPostingDTO(posting entity.PostingData, cursor string) PostingDTO {
	entries := make([]EntryDTO, 0, len(posting.Entries))
	for _, entry := range posting.Entries {
		entries = append(entries, ToEntryDTO(entry))
	}
	return PostingDTO{
		ID:          posting.ID.String(),
		TenantID:    posting.TenantID.String(),
		LedgerID:    posting.LedgerID.String(),
		Operation:   posting.Operation,
		Reference:   posting.ExternalReference,
		Description: posting.Description,
		Entries:     entries,
		EffectiveAt: posting.EffectiveAt,
		RecordedAt:  posting.RecordedAt,
		Cursor:      cursor,
	}
}
