package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TransferRequest moves value between two same-tenant accounts. Immediate
// transfers check funds at execution; scheduled transfers persist PENDING and
// enforce funds atomically at execution. Recurrence expands as
// root:{occurrence}; FX legs are built by the domain service.
type TransferRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	Source         valueobject.AccountID
	Dest           valueobject.AccountID
	AssetCode      valueobject.AssetCode
	AmountMinor    int64
	ExecuteAt      time.Time
	Recurrence     string
	FXRatePresent  bool
	IdempotencyKey string
	Actor          string
}

// TransferResult is the durable outcome of one accepted transfer.
type TransferResult struct {
	TransferID string
	Status     string
	Cursor     string
}

// BatchTransferRequest validates and persists a batch atomically at intake;
// items then execute independently (sibling failure never rolls back).
type BatchTransferRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	Items          []TransferRequest
	IdempotencyKey string
	Actor          string
}

// BatchTransferResult acknowledges intake with per-item idempotency roots.
type BatchTransferResult struct {
	BatchID       string
	TotalItems    int
	AcceptedItems int
	Cursor        string
}

// BatchItemStatus is the execution state of one batch item.
type BatchItemStatus struct {
	Index      int
	TransferID string
	Status     string
	ErrorCode  string
}

// BatchStatusQuery reads one batch with its item states.
type BatchStatusQuery struct {
	TenantID valueobject.TenantID
	BatchID  string
}

// BatchStatusResult reports intake totals plus per-item execution states.
type BatchStatusResult struct {
	BatchID   string
	State     string
	Succeeded int
	Failed    int
	Items     []BatchItemStatus
	Cursor    string
}

// TransferQuery reads one transfer by tenant + ID. Strong read. Actor
// carries the canceling identity for CancelTransfer; plain reads ignore it.
type TransferQuery struct {
	TenantID   valueobject.TenantID
	TransferID string
	Actor      string
}

// TransferView is one transfer with its postings linkage and cursor.
type TransferView struct {
	TransferID string
	Status     string
	PostingID  valueobject.PostingID
	Cursor     string
}

// TransferFilter pages transfers with optional account/status bounds.
type TransferFilter struct {
	TenantID  valueobject.TenantID
	AccountID valueobject.AccountID
	Status    string
	Cursor    string
	Limit     int
}

// TransferPage is one point-in-time page of transfers.
type TransferPage struct {
	Transfers  []TransferView
	NextCursor string
}

// Transfer lifecycle states.
const (
	TransferPending   = "PENDING"
	TransferCompleted = "COMPLETED"
	TransferFailed    = "FAILED"
	TransferCanceled  = "CANCELED"
)

// Batch lifecycle states.
const (
	BatchReceived  = "RECEIVED"
	BatchCompleted = "COMPLETED"
	BatchPartial   = "PARTIAL"
)

// TransferRecord is the durable transfer intent: parameters, state, and the
// committed posting link once executed.
type TransferRecord struct {
	ID          string
	TenantID    valueobject.TenantID
	LedgerID    valueobject.LedgerID
	Source      valueobject.AccountID
	Dest        valueobject.AccountID
	AssetCode   valueobject.AssetCode
	AmountMinor int64
	Status      string
	ExecuteAt   time.Time
	Recurrence  string
	PostingID   valueobject.PostingID
	ErrorCode   string
	CreatedAt   time.Time
}

// BatchRecord is the durable batch intake with its completion state.
type BatchRecord struct {
	ID        string
	TenantID  valueobject.TenantID
	LedgerID  valueobject.LedgerID
	State     string
	CreatedAt time.Time
}

// BatchItem is one batch member with its execution outcome.
type BatchItem struct {
	BatchID    string
	Index      int
	TransferID string
	Status     string
	ErrorCode  string
}

// TransferListFilter pages transfer records with optional bounds.
type TransferListFilter struct {
	TenantID  valueobject.TenantID
	AccountID valueobject.AccountID
	Status    string
	Cursor    string
	Limit     int
}

// TransferCommandUseCases defines the mutating operations on transfers. Strong writes.
type TransferCommandUseCases interface {
	// CreateTransfer accepts an immediate or scheduled transfer. Strong write.
	CreateTransfer(ctx context.Context, req TransferRequest) (TransferResult, error)
	// CancelTransfer cancels a PENDING transfer. Strong write.
	CancelTransfer(ctx context.Context, query TransferQuery) (TransferResult, error)
	// CreateBatchTransfer validates and persists a batch at intake. Strong write.
	CreateBatchTransfer(ctx context.Context, req BatchTransferRequest) (BatchTransferResult, error)
}

// TransferQueryUseCases defines the read operations on transfers.
type TransferQueryUseCases interface {
	// GetBatchStatus reports batch + item states. Point-in-time read.
	GetBatchStatus(ctx context.Context, query BatchStatusQuery) (BatchStatusResult, error)
	// GetTransfer returns one transfer. Strong read.
	GetTransfer(ctx context.Context, query TransferQuery) (TransferView, error)
	// ListTransfers pages transfers by filter. Point-in-time page.
	ListTransfers(ctx context.Context, filter TransferFilter) (TransferPage, error)
}

// TransferUseCases is the composite inbound transfer surface (implemented in E06-T03).
// Batch intake is atomic; item execution reuses the single-transfer path with
// {batch}:{index} idempotency keys.
type TransferUseCases interface {
	TransferCommandUseCases
	TransferQueryUseCases
}
