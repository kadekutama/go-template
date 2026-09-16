package dto

// PayoutDTO is one payout on the edge with its settlement state.
type PayoutDTO struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AssetCode   string `json:"asset_code"`
	AmountMinor int64  `json:"amount_minor"`
	Cursor      string `json:"cursor"`
}

// ToPayoutDTO maps one payout result with its asset and amount.
func ToPayoutDTO(id, status, asset string, amount int64, cursor string) PayoutDTO {
	return PayoutDTO{ID: id, Status: status, AssetCode: asset, AmountMinor: amount, Cursor: cursor}
}

// TopupDTO is one top-up on the edge.
type TopupDTO struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	AssetCode   string `json:"asset_code"`
	AmountMinor int64  `json:"amount_minor"`
	Cursor      string `json:"cursor"`
}

// ToTopupDTO maps one top-up result with its asset and amount.
func ToTopupDTO(id, status, asset string, amount int64, cursor string) TopupDTO {
	return TopupDTO{ID: id, Status: status, AssetCode: asset, AmountMinor: amount, Cursor: cursor}
}
