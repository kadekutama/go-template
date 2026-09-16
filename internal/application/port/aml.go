package port

import (
	"context"
	"time"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ScreeningSubject is the party under review. Only identifiers cross this
// boundary; evidentiary documents travel through object storage (E10).
type ScreeningSubject struct {
	TenantID valueobject.TenantID
	PartyID  string
	FullName string
	Country  string
}

// ScreeningVerdict is the provider's review answer with its reference.
type ScreeningVerdict struct {
	ReferenceID string
	RiskLevel   string
	Matched     bool
	DecidedAt   time.Time
}

// ScreeningProvider is the AML/KYC screening boundary (implemented in E10).
// Screening is advisory: recording decisions and exports stays with the
// compliance use cases (E06-T05). Provider latency is bounded per call;
// retries reuse the caller reference and never duplicate reviews.
type ScreeningProvider interface {
	// Screen reviews one party. Point-in-time advisory fact.
	Screen(ctx context.Context, subject ScreeningSubject, reference string, timeout time.Duration) (ScreeningVerdict, error)
}
