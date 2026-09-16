package dto

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// TenantSettingsDTO mirrors the tenant settings on the edge.
type TenantSettingsDTO struct {
	DefaultCurrency       string   `json:"default_currency"`
	Timezone              string   `json:"timezone"`
	EnabledFeatures       []string `json:"enabled_features"`
	EnabledPaymentMethods []string `json:"enabled_payment_methods"`
}

// TenantDTO is one tenant on the edge. Balances and secrets are never
// included: this is identity and settings only.
type TenantDTO struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Region   string            `json:"region"`
	Status   string            `json:"status"`
	Settings TenantSettingsDTO `json:"settings"`
	Version  int64             `json:"version"`
	Cursor   string            `json:"cursor"`
}

// ToTenantDTO maps one stored tenant plus its read cursor to the edge.
func ToTenantDTO(tenant entity.TenantData, cursor string) TenantDTO {
	return TenantDTO{
		ID: tenant.ID.String(), Name: tenant.Name, Region: tenant.Region, Status: string(tenant.Status),
		Settings: TenantSettingsDTO{
			DefaultCurrency:       string(tenant.Settings.DefaultCurrency),
			Timezone:              tenant.Settings.Timezone,
			EnabledFeatures:       tenant.Settings.EnabledFeatures,
			EnabledPaymentMethods: tenant.Settings.EnabledPaymentMethods,
		},
		Version: tenant.Version, Cursor: cursor,
	}
}
