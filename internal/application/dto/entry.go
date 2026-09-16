// Package dto owns the core ledger edge shapes: transport-neutral structs
// exposing integer minor units, asset codes, as-of times, and ledger cursors.
// Extended bounded-context DTOs live in E06-T02–T05/T11; they map onto the
// port contracts, never around them.
package dto

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// EntryDTO is one journal line on the edge.
type EntryDTO struct {
	ID          string `json:"id"`
	PostingID   string `json:"posting_id"`
	AccountID   string `json:"account_id"`
	Side        string `json:"side"`
	AmountMinor int64  `json:"amount_minor"`
	AssetCode   string `json:"asset_code"`
	AccountSeq  int64  `json:"account_seq"`
}

// ToEntryDTO maps one domain entry to the edge. No workflow state is carried:
// entries are immutable accounting facts.
func ToEntryDTO(entry entity.Entry) EntryDTO {
	return EntryDTO{
		ID:          entry.ID.String(),
		PostingID:   entry.PostingID.String(),
		AccountID:   entry.AccountID.String(),
		Side:        string(entry.Side),
		AmountMinor: entry.AmountMinor,
		AssetCode:   string(entry.AssetCode),
		AccountSeq:  entry.AccountSeq,
	}
}
