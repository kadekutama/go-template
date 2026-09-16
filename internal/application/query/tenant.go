package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// TenantQueryServiceParams encapsulates dependencies for TenantQueryService.
type TenantQueryServiceParams struct {
	Tenants command.TenantStore
}

// TenantQueryService serves tenant reads for the edge by querying the tenant store directly.
type TenantQueryService struct {
	tenants command.TenantStore
}

// NewTenantQueryService creates an encapsulated TenantQueryService with validated dependencies.
func NewTenantQueryService(params TenantQueryServiceParams) *TenantQueryService {
	return &TenantQueryService{
		tenants: params.Tenants,
	}
}

var _ port.TenantQueryUseCases = (*TenantQueryService)(nil)

// GetTenant returns one tenant. Strong read.
func (s *TenantQueryService) GetTenant(ctx context.Context, query port.TenantQuery) (port.TenantResult, error) {
	tenant, err := s.tenants.FindByID(ctx, query.TenantID)
	if err != nil {
		return port.TenantResult{}, err
	}
	return port.TenantResult{Tenant: tenant}, nil
}

// ListTenants returns all tenants (platform-scoped, edge-gated). Strong read.
func (s *TenantQueryService) ListTenants(ctx context.Context) ([]entity.TenantData, error) {
	return s.tenants.ListTenants(ctx)
}
