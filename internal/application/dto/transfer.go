package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
)

// TransferDTO is one transfer intent on the edge: string IDs, integer minor
// units, timestamps, status, and cursor — everything gRPC/GraphQL need
// without a second lookup (protocol-parity note, E06-T03).
type TransferDTO struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	LedgerID    string     `json:"ledger_id"`
	Source      string     `json:"source"`
	Dest        string     `json:"dest"`
	AssetCode   string     `json:"asset_code"`
	AmountMinor int64      `json:"amount_minor"`
	Status      string     `json:"status"`
	ExecuteAt   *time.Time `json:"execute_at,omitempty"`
	Recurrence  string     `json:"recurrence,omitempty"`
	PostingID   string     `json:"posting_id,omitempty"`
	ErrorCode   string     `json:"error_code,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	Cursor      string     `json:"cursor"`
}

// ToTransferDTO maps one stored transfer record plus its read cursor.
func ToTransferDTO(record command.TransferRecord, cursor string) TransferDTO {
	dto := TransferDTO{
		ID: record.ID, TenantID: record.TenantID.String(), LedgerID: record.LedgerID.String(),
		Source: string(record.Source), Dest: string(record.Dest), AssetCode: string(record.AssetCode),
		AmountMinor: record.AmountMinor, Status: record.Status, Recurrence: record.Recurrence,
		PostingID: string(record.PostingID), ErrorCode: record.ErrorCode,
		CreatedAt: record.CreatedAt, Cursor: cursor,
	}
	if !record.ExecuteAt.IsZero() {
		at := record.ExecuteAt
		dto.ExecuteAt = &at
	}
	return dto
}

// ToTransferViewDTO maps one transfer view (no timestamps stored on views).
func ToTransferViewDTO(view port.TransferView) TransferDTO {
	return TransferDTO{
		ID: view.TransferID, Status: view.Status, PostingID: string(view.PostingID), Cursor: view.Cursor,
	}
}
