package service

import (
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ChildBalance is one tenant balance input for consolidation.
type ChildBalance struct {
	TenantID    string
	AssetCode   valueobject.AssetCode
	AmountMinor int64
}

// ConsolidationRequest rolls child balances into a base-asset total.
// Rates maps child asset → FxRate converting that asset to BaseAsset at At.
// Same-asset children need no rate entry.
type ConsolidationRequest struct {
	BaseAsset valueobject.AssetCode
	Children  []ChildBalance
	Rates     map[valueobject.AssetCode]valueobject.FxRate
	At        time.Time
}

// ConsolidateBalances sums converted children with checked arithmetic.
// It uses E03-T05 Convert (half-up, no floats) for cross-asset legs.
func ConsolidateBalances(req ConsolidationRequest) (int64, error) {
	if strings.TrimSpace(string(req.BaseAsset)) == "" {
		return 0, entity.NewError("CONSOLIDATION_ASSET_REQUIRED", "consolidation requires a base asset")
	}
	if len(req.Children) == 0 {
		return 0, entity.NewError("CONSOLIDATION_EMPTY", "consolidation requires at least one child balance")
	}
	if req.At.IsZero() {
		return 0, entity.NewError("CONSOLIDATION_TIME_REQUIRED", "consolidation time is required")
	}
	var total int64
	for _, child := range req.Children {
		converted, err := convertChildBalance(child, req.BaseAsset, req.Rates, req.At)
		if err != nil {
			return 0, err
		}
		next, ok := checkedAdd(total, converted)
		if !ok {
			return 0, entity.NewError("CONSOLIDATION_OVERFLOW", "consolidation total overflowed")
		}
		total = next
	}
	return total, nil
}

// convertChildBalance validates one child leg and converts it to the base
// asset. Same-asset legs pass through; cross-asset legs require a supplied
// rate whose pair converts child → base.
func convertChildBalance(child ChildBalance, base valueobject.AssetCode, rates map[valueobject.AssetCode]valueobject.FxRate, at time.Time) (int64, error) {
	if strings.TrimSpace(child.TenantID) == "" {
		return 0, entity.NewError("CONSOLIDATION_TENANT_REQUIRED", "consolidation child requires a tenant id")
	}
	if strings.TrimSpace(string(child.AssetCode)) == "" {
		return 0, entity.NewError("CONSOLIDATION_ASSET_REQUIRED", "consolidation child requires an asset code")
	}
	if child.AmountMinor < 0 {
		return 0, entity.NewError("CONSOLIDATION_AMOUNT_INVALID", "consolidation amount must be non-negative")
	}
	if child.AssetCode == base {
		return child.AmountMinor, nil
	}
	rate, ok := rates[child.AssetCode]
	if !ok {
		return 0, entity.NewError("FX_RATE_MISSING", "consolidation requires an FX rate for child asset")
	}
	if rate.Pair.Base != child.AssetCode || rate.Pair.Quote != base {
		return 0, entity.NewError("FX_RATE_MISMATCH", "fx rate pair must convert child asset to base asset")
	}
	return Convert(child.AmountMinor, rate, at)
}

// HasTransferGrant reports whether an explicit per-child grant exists.
// Grants are keyed by child tenant ID; empty IDs never pass.
func HasTransferGrant(grants map[string]bool, childTenantID string) bool {
	if strings.TrimSpace(childTenantID) == "" {
		return false
	}
	return grants[strings.TrimSpace(childTenantID)]
}

// ValidateTransferGrant rejects cross-child transfers without an explicit
// per-child grant (no implicit parent inheritance).
func ValidateTransferGrant(grants map[string]bool, childTenantID string) error {
	if strings.TrimSpace(childTenantID) == "" {
		return entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id")
	}
	if !HasTransferGrant(grants, childTenantID) {
		return entity.NewError("HIERARCHY_GRANT_REQUIRED", "cross-child transfer requires an explicit hierarchy grant")
	}
	return nil
}

// HasReadGrant reports whether an explicit per-child read grant exists for parent views.
func HasReadGrant(grants map[string]bool, childTenantID string) bool {
	return HasTransferGrant(grants, childTenantID)
}

// ValidateReadGrant rejects parent view access without an explicit per-child grant
// (enforces features §5: parent read access requires explicit grant per child).
func ValidateReadGrant(grants map[string]bool, childTenantID string) error {
	if strings.TrimSpace(childTenantID) == "" {
		return entity.NewError("HIERARCHY_ID_REQUIRED", "hierarchy grant requires a child tenant id")
	}
	if !HasReadGrant(grants, childTenantID) {
		return entity.NewError("HIERARCHY_GRANT_REQUIRED", "parent read access requires an explicit hierarchy grant")
	}
	return nil
}
