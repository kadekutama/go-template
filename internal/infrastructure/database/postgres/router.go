package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Residency pins a tenant to a region pool (E05-T03 pointers, E07-T08
// enforcement). Unknown regions fail closed: never default-region fallback.
type Residency struct {
	TenantID valueobject.TenantID
	Region   string
}

// AuditFunc records residency decisions for data-movement proof.
type AuditFunc func(ctx context.Context, tenant valueobject.TenantID, region string, at time.Time)

// Router resolves region pools at request scope.
type Router struct {
	pools map[string]*gorm.DB
	audit AuditFunc
}

// RouterParams carries constructor dependencies.
type RouterParams struct {
	// Pools maps region name to pool; must be non-empty.
	Pools map[string]*gorm.DB
	// Audit records decisions; nil disables recording (routing unaffected).
	Audit AuditFunc
}

// NewRouter builds the registry; empty pools are rejected.
func NewRouter(params RouterParams) (*Router, error) {
	if len(params.Pools) == 0 {
		return nil, fmt.Errorf("postgres: router needs at least one region pool")
	}

	for region, pool := range params.Pools {
		if region == "" || pool == nil {
			return nil, fmt.Errorf("postgres: router needs a named pool per region")
		}
	}

	return &Router{pools: params.Pools, audit: params.Audit}, nil
}

// Resolve returns the pool for the tenant's region or fails closed.
func (r *Router) Resolve(ctx context.Context, residency Residency) (*gorm.DB, error) {
	if residency.TenantID.String() == "" {
		return nil, fmt.Errorf("postgres: tenant is required")
	}

	if residency.Region == "" {
		return nil, fmt.Errorf("postgres: residency region is required")
	}

	pool, ok := r.pools[residency.Region]
	if !ok || pool == nil {
		return nil, fmt.Errorf("postgres: unknown region %s", residency.Region)
	}

	if r.audit != nil {
		r.audit(ctx, residency.TenantID, residency.Region, time.Now())
	}

	return pool, nil
}
