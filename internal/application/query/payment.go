package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// PaymentQueryServiceParams encapsulates dependencies for PaymentQueryService.
type PaymentQueryServiceParams struct {
	Payments port.PaymentQueryUseCases
}

// PaymentQueryService serves payment reads for the edge by delegating to the
// payment use cases. Strong reads; charging flows never read through here.
type PaymentQueryService struct {
	payments port.PaymentQueryUseCases
}

// NewPaymentQueryService creates an encapsulated PaymentQueryService with validated dependencies.
func NewPaymentQueryService(params PaymentQueryServiceParams) *PaymentQueryService {
	return &PaymentQueryService{
		payments: params.Payments,
	}
}

// GetIntent returns one payment intent. Strong read.
func (s *PaymentQueryService) GetIntent(ctx context.Context, query port.PaymentQuery) (port.PaymentIntentResult, error) {
	return s.payments.GetIntent(ctx, query)
}

// ListIntents returns one tenant's intents, newest first (bounded). Strong read.
func (s *PaymentQueryService) ListIntents(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.PaymentIntentResult, error) {
	return s.payments.ListIntents(ctx, tenant, limit)
}
