package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PayoutQueryServiceParams encapsulates dependencies for PayoutQueryService.
type PayoutQueryServiceParams struct {
	Payments port.PaymentQueryUseCases
}

// PayoutQueryService serves payout reads for the edge by delegating to the
// payment use cases. Strong reads.
type PayoutQueryService struct {
	payments port.PaymentQueryUseCases
}

// NewPayoutQueryService creates an encapsulated PayoutQueryService with validated dependencies.
func NewPayoutQueryService(params PayoutQueryServiceParams) *PayoutQueryService {
	return &PayoutQueryService{
		payments: params.Payments,
	}
}

// GetPayout returns one payout. Strong read.
func (s *PayoutQueryService) GetPayout(ctx context.Context, query port.PaymentQuery) (port.PayoutResult, error) {
	return s.payments.GetPayout(ctx, query)
}

// ListPayouts returns one tenant's payouts, newest first (bounded). Strong read.
func (s *PayoutQueryService) ListPayouts(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PayoutResult, error) {
	return s.payments.ListPayouts(ctx, tenant, limit)
}

// ListTopups returns one tenant's top-ups, newest first (bounded). Strong read.
func (s *PayoutQueryService) ListTopups(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.TopupResult, error) {
	return s.payments.ListTopups(ctx, tenant, limit)
}
