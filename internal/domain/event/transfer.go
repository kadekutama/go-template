package event

import (
	"errors"
	"time"
)

// TransferCreatedPayload records an accepted transfer command.
type TransferCreatedPayload struct {
	TransferID    string `json:"transfer_id"`
	TenantID      string `json:"tenant_id"`
	SourceAccount string `json:"source_account"`
	DestAccount   string `json:"dest_account"`
	AmountMinor   int64  `json:"amount_minor"`
	AssetCode     string `json:"asset_code"`
}

// NewTransferCreated builds transfer.created.v1.
func NewTransferCreated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferCreatedPayload, meta EventMetadata) (TypedEvent[TransferCreatedPayload], error) {
	if err := requireIDs(map[string]string{fieldTransferID: payload.TransferID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TransferCreatedPayload]{}, err
	}
	if payload.AmountMinor <= 0 {
		return TypedEvent[TransferCreatedPayload]{}, errors.New("event: amount_minor must be positive")
	}
	return newTyped("transfer.created.v1", eventID, aggregateID, "Transfer", occurredAt, version, seq, payload, meta)
}

// TransferCompletedPayload records posted transfer entries.
type TransferCompletedPayload struct {
	TransferID string `json:"transfer_id"`
	TenantID   string `json:"tenant_id"`
	PostingID  string `json:"posting_id"`
}

// NewTransferCompleted builds transfer.completed.v1.
func NewTransferCompleted(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferCompletedPayload, meta EventMetadata) (TypedEvent[TransferCompletedPayload], error) {
	if err := requireIDs(map[string]string{fieldTransferID: payload.TransferID, fieldTenantID: payload.TenantID, fieldPostingID: payload.PostingID}); err != nil {
		return TypedEvent[TransferCompletedPayload]{}, err
	}
	return newTyped("transfer.completed.v1", eventID, aggregateID, "Transfer", occurredAt, version, seq, payload, meta)
}

// TransferFailedPayload records a validation or balance failure.
type TransferFailedPayload struct {
	TransferID string `json:"transfer_id"`
	TenantID   string `json:"tenant_id"`
	Code       string `json:"code"`
	Message    string `json:"message"`
}

// NewTransferFailed builds transfer.failed.v1.
func NewTransferFailed(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferFailedPayload, meta EventMetadata) (TypedEvent[TransferFailedPayload], error) {
	if err := requireIDs(map[string]string{fieldTransferID: payload.TransferID, fieldTenantID: payload.TenantID, fieldCode: payload.Code}); err != nil {
		return TypedEvent[TransferFailedPayload]{}, err
	}
	return newTyped("transfer.failed.v1", eventID, aggregateID, "Transfer", occurredAt, version, seq, payload, meta)
}

// TransferCanceledPayload records a user-cancelled pending transfer.
type TransferCanceledPayload struct {
	TransferID  string `json:"transfer_id"`
	TenantID    string `json:"tenant_id"`
	CancelledBy string `json:"cancelled_by"`
}

// NewTransferCanceled builds transfer.canceled.v1 (American spelling).
func NewTransferCanceled(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferCanceledPayload, meta EventMetadata) (TypedEvent[TransferCanceledPayload], error) {
	if err := requireIDs(map[string]string{fieldTransferID: payload.TransferID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TransferCanceledPayload]{}, err
	}
	return newTyped("transfer.canceled.v1", eventID, aggregateID, "Transfer", occurredAt, version, seq, payload, meta)
}

// TransferBatchReceivedPayload records accepted batch intake for fan-out.
type TransferBatchReceivedPayload struct {
	BatchID   string `json:"batch_id"`
	TenantID  string `json:"tenant_id"`
	ItemCount int    `json:"item_count"`
}

// NewTransferBatchReceived builds transfer.batch.received.v1.
func NewTransferBatchReceived(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferBatchReceivedPayload, meta EventMetadata) (TypedEvent[TransferBatchReceivedPayload], error) {
	if err := requireIDs(map[string]string{"batch_id": payload.BatchID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TransferBatchReceivedPayload]{}, err
	}
	if payload.ItemCount <= 0 {
		return TypedEvent[TransferBatchReceivedPayload]{}, errors.New("event: item_count must be positive")
	}
	return newTyped("transfer.batch.received.v1", eventID, aggregateID, "TransferBatch", occurredAt, version, seq, payload, meta)
}

// TransferBatchCompletedPayload records a fully settled batch.
type TransferBatchCompletedPayload struct {
	BatchID        string `json:"batch_id"`
	TenantID       string `json:"tenant_id"`
	CompletedCount int    `json:"completed_count"`
	FailedCount    int    `json:"failed_count"`
}

// NewTransferBatchCompleted builds transfer.batch.completed.v1.
func NewTransferBatchCompleted(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload TransferBatchCompletedPayload, meta EventMetadata) (TypedEvent[TransferBatchCompletedPayload], error) {
	if err := requireIDs(map[string]string{"batch_id": payload.BatchID, fieldTenantID: payload.TenantID}); err != nil {
		return TypedEvent[TransferBatchCompletedPayload]{}, err
	}
	return newTyped("transfer.batch.completed.v1", eventID, aggregateID, "TransferBatch", occurredAt, version, seq, payload, meta)
}
