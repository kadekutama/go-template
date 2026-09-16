package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
)

// BatchItemDTO is one batch member outcome on the edge.
type BatchItemDTO struct {
	Index      int    `json:"index"`
	TransferID string `json:"transfer_id"`
	Status     string `json:"status"`
	ErrorCode  string `json:"error_code,omitempty"`
}

// BatchDTO is one batch intake with its completion state and item outcomes.
type BatchDTO struct {
	ID         string         `json:"id"`
	TenantID   string         `json:"tenant_id"`
	LedgerID   string         `json:"ledger_id"`
	State      string         `json:"state"`
	TotalItems int            `json:"total_items"`
	Succeeded  int            `json:"succeeded"`
	Failed     int            `json:"failed"`
	Items      []BatchItemDTO `json:"items"`
	CreatedAt  time.Time      `json:"created_at"`
	Cursor     string         `json:"cursor"`
}

// ToBatchDTO maps one stored batch with item outcomes and read cursor.
func ToBatchDTO(batch command.BatchRecord, items []command.BatchItem, succeeded, failed int, cursor string) BatchDTO {
	members := make([]BatchItemDTO, 0, len(items))
	for _, item := range items {
		members = append(members, BatchItemDTO{
			Index: item.Index, TransferID: item.TransferID, Status: item.Status, ErrorCode: item.ErrorCode,
		})
	}
	return BatchDTO{
		ID: batch.ID, TenantID: batch.TenantID.String(), LedgerID: batch.LedgerID.String(),
		State: batch.State, TotalItems: len(items), Succeeded: succeeded, Failed: failed,
		Items: members, CreatedAt: batch.CreatedAt, Cursor: cursor,
	}
}

// ToBatchStatusDTO maps one batch status view.
func ToBatchStatusDTO(status port.BatchStatusResult) BatchDTO {
	members := make([]BatchItemDTO, 0, len(status.Items))
	for _, item := range status.Items {
		members = append(members, BatchItemDTO{
			Index: item.Index, TransferID: item.TransferID, Status: item.Status, ErrorCode: item.ErrorCode,
		})
	}
	return BatchDTO{
		ID: status.BatchID, State: status.State, TotalItems: len(status.Items),
		Succeeded: status.Succeeded, Failed: status.Failed, Items: members,
	}
}
