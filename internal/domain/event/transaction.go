package event

import (
	"errors"
	"time"
)

// EntryPayload is one immutable side of a committed posting.
type EntryPayload struct {
	EntryID       string `json:"entry_id"`
	AccountID     string `json:"account_id"`
	AccountNumber string `json:"account_number"`
	Direction     string `json:"direction"`
	AmountMinor   int64  `json:"amount_minor"`
	AssetCode     string `json:"asset_code"`
	AccountSeq    int64  `json:"account_sequence"`
}

// TransactionPostedPayload is the public/API face of an accepted immutable
// Posting, including its complete entry set for audit and reconciliation.
type TransactionPostedPayload struct {
	PostingID   string         `json:"posting_id"`
	TenantID    string         `json:"tenant_id"`
	LedgerID    string         `json:"ledger_id"`
	Operation   string         `json:"operation"`
	Description string         `json:"description"`
	Reference   string         `json:"reference"`
	Entries     []EntryPayload `json:"entries"`
	RecordedAt  time.Time      `json:"recorded_at"`
}

// NewTransactionPosted builds transaction.posted.v1.
func NewTransactionPosted(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransactionPostedPayload, meta EventMetadata) (TypedEvent[TransactionPostedPayload], error) {
	if err := requireIDs(map[string]string{fieldPostingID: payload.PostingID, fieldTenantID: payload.TenantID, fieldLedgerID: payload.LedgerID}); err != nil {
		return TypedEvent[TransactionPostedPayload]{}, err
	}
	if len(payload.Entries) < 2 {
		return TypedEvent[TransactionPostedPayload]{}, errors.New("event: posted transaction requires at least two entries")
	}
	entries := make([]EntryPayload, len(payload.Entries))
	copy(entries, payload.Entries)
	payload.Entries = entries
	return newTyped("transaction.posted.v1", eventID, aggregateID, "Posting", occurredAt, version, seq, payload, meta)
}

// TransactionReversedPayload records a newly committed reversal posting. The
// original posting is unchanged.
type TransactionReversedPayload struct {
	PostingID           string            `json:"posting_id"`
	TenantID            string            `json:"tenant_id"`
	OriginalPostingID   string            `json:"original_posting_id"`
	ReversalType        string            `json:"reversal_type"`
	ReversedAmountMinor int64             `json:"reversed_amount_minor"`
	ReversedEntries     []EntryPayload    `json:"reversed_entries"`
	Reason              string            `json:"reason"`
	ReversedAt          time.Time         `json:"reversed_at"`
	Metadata            map[string]string `json:"metadata,omitempty"`
}

// NewTransactionReversed builds transaction.reversed.v1.
func NewTransactionReversed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransactionReversedPayload, meta EventMetadata) (TypedEvent[TransactionReversedPayload], error) {
	if err := requireIDs(map[string]string{fieldPostingID: payload.PostingID, fieldTenantID: payload.TenantID, "original_posting_id": payload.OriginalPostingID}); err != nil {
		return TypedEvent[TransactionReversedPayload]{}, err
	}
	if payload.Reason == "" {
		return TypedEvent[TransactionReversedPayload]{}, errors.New("event: reason is required")
	}
	entries := make([]EntryPayload, len(payload.ReversedEntries))
	copy(entries, payload.ReversedEntries)
	payload.ReversedEntries = entries
	if payload.Metadata != nil {
		md := make(map[string]string, len(payload.Metadata))
		for k, v := range payload.Metadata {
			md[k] = v
		}
		payload.Metadata = md
	}
	return newTyped("transaction.reversed.v1", eventID, aggregateID, "Posting", occurredAt, version, seq, payload, meta)
}

// TransactionFailedPayload is a workflow notification for a rejected
// transaction. It never creates or mutates ledger entries.
type TransactionFailedPayload struct {
	AttemptID string `json:"attempt_id"`
	TenantID  string `json:"tenant_id"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// NewTransactionFailed builds transaction.failed.v1.
func NewTransactionFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransactionFailedPayload, meta EventMetadata) (TypedEvent[TransactionFailedPayload], error) {
	if err := requireIDs(map[string]string{"attempt_id": payload.AttemptID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[TransactionFailedPayload]{}, err
	}
	return newTyped("transaction.failed.v1", eventID, aggregateID, "Posting", occurredAt, version, seq, payload, meta)
}

// TransactionPendingPayload tracks an asynchronously created transaction. It
// never creates or mutates ledger entries.
type TransactionPendingPayload struct {
	AttemptID string `json:"attempt_id"`
	TenantID  string `json:"tenant_id"`
	Operation string `json:"operation"`
	Reference string `json:"reference"`
}

// NewTransactionPending builds transaction.pending.v1.
func NewTransactionPending(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransactionPendingPayload, meta EventMetadata) (TypedEvent[TransactionPendingPayload], error) {
	if err := requireIDs(map[string]string{"attempt_id": payload.AttemptID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TransactionPendingPayload]{}, err
	}
	return newTyped("transaction.pending.v1", eventID, aggregateID, "Posting", occurredAt, version, seq, payload, meta)
}
