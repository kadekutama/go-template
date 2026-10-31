package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// FxRateProvider fetches fixed-point conversion rates. Rates are fetched
// facts: staleness (TTL) is enforced by callers through IsStale, and floats
// never cross this boundary. Fetch failures are retryable caller-side with
// backoff.
//
// GetRate returns the freshest rate the adapter holds: a cache-fresh rate, or
// a newly fetched one. When the provider or its egress path is unavailable,
// GetRate MAY return the last-known rate whose QuotedAt/TTL make IsStale(at)
// true; it MUST NOT hide that staleness behind a re-stamped timestamp.
// Callers MUST inspect IsStale before money movement (Convert rejects stale
// rates) and MUST NOT treat staleness as absence.
type FxRateProvider interface {
	// GetRate returns the freshest fixed-point rate for base→quote as of at.
	// A zero at means the current instant. Point-in-time fact.
	GetRate(ctx context.Context, base, quote valueobject.AssetCode, at time.Time) (valueobject.FxRate, error)
}
