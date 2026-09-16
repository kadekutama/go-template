package dto

// PaymentIntentDTO is one payment intent on the edge.
type PaymentIntentDTO struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Status      string `json:"status"`
	AssetCode   string `json:"asset_code"`
	AmountMinor int64  `json:"amount_minor"`
	Cursor      string `json:"cursor"`
}

// ToPaymentIntentDTO maps one intent result with its tenant and asset.
func ToPaymentIntentDTO(id, tenant, status, asset string, amount int64, cursor string) PaymentIntentDTO {
	return PaymentIntentDTO{
		ID: id, TenantID: tenant, Status: status, AssetCode: asset, AmountMinor: amount, Cursor: cursor,
	}
}
