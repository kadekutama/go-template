package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// FxRateProvider fetches fixed-point conversion rates. Rates are fetched
// facts: staleness (TTL) is enforced by callers through IsStale, and floats
// never cross this boundary. Fetch failures are retryable caller-side with
// backoff; callers MUST NOT fall back to stale rates silently.
type FxRateProvider interface {
	// GetRate returns a fresh fixed-point rate for base→quote. Point-in-time fact.
	GetRate(ctx context.Context, base, quote valueobject.AssetCode, at time.Time) (valueobject.FxRate, error)
}
