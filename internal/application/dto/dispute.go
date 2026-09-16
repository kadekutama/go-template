package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
)

// DisputeDTO is one dispute on the edge with its deadline, fee, and outcome
// linkage.
type DisputeDTO struct {
	ID            string    `json:"id"`
	PaymentID     string    `json:"payment_id"`
	Network       string    `json:"network"`
	AmountMinor   int64     `json:"amount_minor"`
	Status        string    `json:"status"`
	EvidenceDueAt time.Time `json:"evidence_due_at"`
	FeeMinor      int64     `json:"fee_minor"`
	Stage         int       `json:"represent_stage"`
	Outcome       string    `json:"outcome,omitempty"`
	Cursor        string    `json:"cursor"`
}

// ToDisputeDTO maps one stored dispute plus read cursor and optional outcome.
func ToDisputeDTO(result port.DisputeResult, outcome string) DisputeDTO {
	dispute := result.Dispute
	return DisputeDTO{
		ID: dispute.ID, PaymentID: dispute.PaymentID, Network: dispute.Network,
		AmountMinor: dispute.AmountMinor, Status: string(dispute.Status),
		EvidenceDueAt: dispute.EvidenceDueAt, FeeMinor: dispute.FeeMinor,
		Stage: dispute.RepresentStage, Outcome: outcome, Cursor: result.Cursor,
	}
}
