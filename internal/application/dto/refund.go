package dto

// RefundDTO is one refund on the edge with its reversal linkage.
type RefundDTO struct {
	ID         string `json:"id"`
	OriginalID string `json:"original_id"`
	Status     string `json:"status"`
	Cursor     string `json:"cursor"`
}

// ToRefundDTO maps one refund result.
func ToRefundDTO(id, original, status, cursor string) RefundDTO {
	return RefundDTO{ID: id, OriginalID: original, Status: status, Cursor: cursor}
}
