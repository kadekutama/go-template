package port

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ProvisionTenantRequest onboards one self-serve tenant with its default
// chart of accounts and API keys, atomically (journeys §2.1).
type ProvisionTenantRequest struct {
	Name           string
	Region         string
	Settings       entity.TenantSettings
	IdempotencyKey string
	Actor          string
}

// TenantResult carries the provisioned tenant with its read cursor.
type TenantResult struct {
	Tenant entity.TenantData
	Cursor string
}

// UpdateTenantSettingsRequest mutates non-identity tenant settings under
// optimistic locking.
type UpdateTenantSettingsRequest struct {
	TenantID        valueobject.TenantID
	Settings        entity.TenantSettings
	ExpectedVersion int64
	IdempotencyKey  string
	Actor           string
}

// TenantQuery reads one tenant by ID. Strong read.
type TenantQuery struct {
	TenantID valueobject.TenantID
}

// TenantCommandUseCases defines the mutating operations on tenants. Strong writes.
type TenantCommandUseCases interface {
	// ProvisionTenant creates a tenant with defaults atomically. Strong write.
	ProvisionTenant(ctx context.Context, req ProvisionTenantRequest) (TenantResult, error)
	// UpdateTenantSettings mutates tenant settings. Strong write.
	UpdateTenantSettings(ctx context.Context, req UpdateTenantSettingsRequest) (TenantResult, error)
}

// TenantQueryUseCases defines the read operations on tenants. Strong reads.
type TenantQueryUseCases interface {
	// GetTenant returns one tenant. Strong read.
	GetTenant(ctx context.Context, query TenantQuery) (TenantResult, error)
	// ListTenants returns all tenants (platform-scoped, edge-gated). Strong read.
	ListTenants(ctx context.Context) ([]entity.TenantData, error)
}

// TenantUseCases is the composite inbound tenancy surface (implemented in E06-T02).
// Isolation follows the E05-T03 contract: every query carries its tenant and
// adapters never return cross-tenant rows.
type TenantUseCases interface {
	TenantCommandUseCases
	TenantQueryUseCases
}
