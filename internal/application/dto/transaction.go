package dto

import (
	"github.com/kadekutama/go-template/internal/application/port"
)

// ReverseTransactionDTO is one linked reversal outcome on the edge.
type ReverseTransactionDTO struct {
	ID         string `json:"id"`
	OriginalID string `json:"original_id"`
	Reason     string `json:"reason"`
	Cursor     string `json:"cursor"`
}

// ToReverseTransactionDTO maps one reversal result with its original link.
func ToReverseTransactionDTO(result port.PostingResult, originalID, reason string) ReverseTransactionDTO {
	return ReverseTransactionDTO{
		ID: string(result.PostingID), OriginalID: originalID, Reason: reason, Cursor: result.Cursor,
	}
}

// TransactionDTO is one generic transaction on the edge: the core posting
// shape plus its reversal link when it is a correction.
type TransactionDTO struct {
	Posting    PostingDTO `json:"posting"`
	ReversalOf string     `json:"reversal_of,omitempty"`
}

// ToTransactionDTO maps one committed posting with its reversal link.
func ToTransactionDTO(posting PostingDTO, reversalOf string) TransactionDTO {
	return TransactionDTO{Posting: posting, ReversalOf: reversalOf}
}
