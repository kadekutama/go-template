package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// RefundQueryServiceParams encapsulates dependencies for RefundQueryService.
type RefundQueryServiceParams struct {
	Payments port.PaymentQueryUseCases
}

// RefundQueryService serves refund reads for the edge by delegating to the
// payment use cases. Strong reads.
type RefundQueryService struct {
	payments port.PaymentQueryUseCases
}

// NewRefundQueryService creates an encapsulated RefundQueryService with validated dependencies.
func NewRefundQueryService(params RefundQueryServiceParams) *RefundQueryService {
	return &RefundQueryService{
		payments: params.Payments,
	}
}

// GetRefund returns one refund. Strong read.
func (s *RefundQueryService) GetRefund(ctx context.Context, query port.PaymentQuery) (port.RefundResult, error) {
	return s.payments.GetRefund(ctx, query)
}

// ListRefunds returns one tenant's refunds, newest first (bounded). Strong read.
func (s *RefundQueryService) ListRefunds(ctx context.Context, tenant valueobject.TenantID, limit int) ([]port.RefundResult, error) {
	return s.payments.ListRefunds(ctx, tenant, limit)
}
