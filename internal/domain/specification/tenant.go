package specification

import (
	"context"
	"strings"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// TenantNameValid passes for 3–64 trimmed, control-free tenant names.
func TenantNameValid() Specification[string] {
	return NewFuncSpec[string]("TENANT_NAME_INVALID", "tenant name is invalid",
		func(_ context.Context, name string) bool {
			return entity.ValidateTenantName(name) == nil
		})
}

// TenantRegionAllowed passes when region is in the caller-supplied allowlist
// (case-insensitive, trimmed). The allowlist is versioned configuration, never
// invented by the domain.
func TenantRegionAllowed(allowed []string) Specification[string] {
	snapshot := append([]string(nil), allowed...)
	return NewFuncSpec[string]("TENANT_REGION_INVALID", "tenant region is not allowed",
		func(_ context.Context, region string) bool {
			trimmed := strings.TrimSpace(region)
			if trimmed == "" || len(snapshot) == 0 {
				return false
			}
			for _, a := range snapshot {
				if strings.EqualFold(strings.TrimSpace(a), trimmed) {
					return true
				}
			}
			return false
		})
}

// TenantSettingsValid passes when settings validate structurally.
func TenantSettingsValid() Specification[entity.TenantSettings] {
	return NewFuncSpec[entity.TenantSettings]("TENANT_SETTINGS_INVALID", "tenant settings are invalid",
		func(_ context.Context, s entity.TenantSettings) bool {
			return s.Validate() == nil
		})
}
