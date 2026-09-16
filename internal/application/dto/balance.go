package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
)

// BalanceDTO is one strongly read balance with its as-of time and cursor.
type BalanceDTO struct {
	AccountID      string    `json:"account_id"`
	AssetCode      string    `json:"asset_code"`
	AvailableMinor int64     `json:"available_minor"`
	AsOf           time.Time `json:"as_of"`
	Cursor         string    `json:"cursor"`
}

// ToBalanceDTO maps one strongly read balance view to the edge.
func ToBalanceDTO(view port.BalanceView) BalanceDTO {
	return BalanceDTO{
		AccountID:      view.AccountID.String(),
		AssetCode:      string(view.AssetCode),
		AvailableMinor: view.AvailableMinor,
		AsOf:           view.AsOf,
		Cursor:         view.Cursor,
	}
}
