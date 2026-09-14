package service

import (
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Convert translates base minor units to quote minor units at a fixed-point
// rate with half-up rounding: (amount × num + den/2) / den. It rejects stale
// rates, invalid shapes, and overflow. No floats are used.
func Convert(amountMinor int64, rate valueobject.FxRate, at time.Time) (int64, error) {
	if err := rate.Validate(); err != nil {
		return 0, entity.NewError("FX_RATE_INVALID", "fx rate is invalid: "+err.Error())
	}
	if amountMinor < 0 {
		return 0, entity.NewError("INVALID_FX_AMOUNT", "fx amount must be non-negative")
	}
	if rate.IsStale(at) {
		return 0, entity.NewError("FX_RATE_STALE", "fx rate is stale")
	}
	hi, ok := checkedMul(amountMinor, rate.Numerator)
	if !ok {
		return 0, entity.NewError("FX_OVERFLOW", "fx conversion overflowed")
	}
	// Half-up: (hi + den/2) / den.
	q, ok := checkedAdd(hi, rate.Denominator/2)
	if !ok {
		return 0, entity.NewError("FX_OVERFLOW", "fx conversion overflowed")
	}
	return q / rate.Denominator, nil
}

// GainLoss computes settlement-vs-authorization quote drift for a base
// amount: positive means gain (settlement quote higher), negative means loss.
// Both rates must share the pair; staleness is the caller's policy (this is
// a pure arithmetic helper over already-accepted rates).
func GainLoss(authorizeQuoteMinor, settleQuoteMinor int64) int64 {
	return settleQuoteMinor - authorizeQuoteMinor
}

// LotTotals is one independently balanced currency lot.
type LotTotals struct {
	Asset   valueobject.AssetCode
	Debits  int64
	Credits int64
}

// LotsBalance enforces per-asset debit == credit for every linked lot.
func LotsBalance(lots []LotTotals) error {
	if len(lots) == 0 {
		return entity.NewError("FX_LOTS_REQUIRED", "fx conversion requires at least one currency lot")
	}
	for _, l := range lots {
		if l.Asset == "" {
			return entity.NewError("FX_LOT_ASSET_REQUIRED", "fx lot requires an asset code")
		}
		if l.Debits <= 0 || l.Credits <= 0 {
			return entity.NewError("INVALID_ENTRY_AMOUNT", "fx lot legs must be positive")
		}
		if l.Debits != l.Credits {
			return entity.NewError("UNBALANCED_TRANSACTION", "fx lot must balance independently")
		}
	}
	return nil
}
