package event

import (
	"errors"
	"time"
)

// FxRateUpdatedPayload records a newly fetched FX rate.
type FxRateUpdatedPayload struct {
	RateID    string    `json:"rate_id"`
	TenantID  string    `json:"tenant_id"`
	FromAsset string    `json:"from_asset"`
	ToAsset   string    `json:"to_asset"`
	Rate      string    `json:"rate"`
	Source    string    `json:"source"`
	QuotedAt  time.Time `json:"quoted_at"`
}

// NewFxRateUpdated builds fx.rate.updated.v1.
func NewFxRateUpdated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload FxRateUpdatedPayload, meta EventMetadata) (TypedEvent[FxRateUpdatedPayload], error) {
	if err := requireIDs(map[string]string{"rate_id": payload.RateID, fieldTenantID: payload.TenantID, "from_asset": payload.FromAsset, "to_asset": payload.ToAsset}); err != nil {
		return TypedEvent[FxRateUpdatedPayload]{}, err
	}
	if payload.Rate == "" {
		return TypedEvent[FxRateUpdatedPayload]{}, errors.New("event: rate is required")
	}
	return newTyped("fx.rate.updated.v1", eventID, aggregateID, "FxRate", occurredAt, version, seq, payload, meta)
}
