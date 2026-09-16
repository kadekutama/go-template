package dto

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// AccountDTO is one ledger account on the edge.
type AccountDTO struct {
	ID        string            `json:"id"`
	TenantID  string            `json:"tenant_id"`
	LedgerID  string            `json:"ledger_id"`
	Number    string            `json:"number"`
	Name      string            `json:"name"`
	Class     string            `json:"class"`
	AssetCode string            `json:"asset_code"`
	Status    string            `json:"status"`
	Purpose   string            `json:"purpose"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Version   int64             `json:"version"`
	Cursor    string            `json:"cursor"`
}

// ToAccountDTO maps one stored account plus its read cursor to the edge.
func ToAccountDTO(account entity.AccountData, cursor string) AccountDTO {
	return AccountDTO{
		ID: account.ID.String(), TenantID: account.TenantID.String(), LedgerID: account.LedgerID.String(),
		Number: account.Number, Name: account.Name, Class: string(account.Class),
		AssetCode: string(account.AssetCode), Status: string(account.Status),
		Purpose: account.Purpose, Metadata: account.Metadata, Version: account.Version, Cursor: cursor,
	}
}
