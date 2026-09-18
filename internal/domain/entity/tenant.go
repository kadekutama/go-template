package entity

import (
	"strings"
	"time"
	"unicode"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TenantStatus is the tenancy lifecycle state.
type TenantStatus string

// Tenant lifecycle states.
const (
	TenantActive    TenantStatus = "ACTIVE"
	TenantSuspended TenantStatus = "SUSPENDED"
	TenantClosed    TenantStatus = "CLOSED"
)

// ParseTenantStatus validates a tenancy lifecycle state.
func ParseTenantStatus(s string) (TenantStatus, error) {
	switch TenantStatus(s) {
	case TenantActive, TenantSuspended, TenantClosed:
		return TenantStatus(s), nil
	default:
		return "", NewError("TENANT_STATUS_INVALID", "tenant status is invalid")
	}
}

// TenantSettings carries per-tenant defaults and feature gates.
type TenantSettings struct {
	DefaultCurrency       valueobject.AssetCode
	Timezone              string
	EnabledFeatures       []string
	EnabledPaymentMethods []string
}

// Validate checks settings shape. Feature/method entries must be non-empty,
// bounded, control-free, and duplicate-free.
func (s TenantSettings) Validate() error {
	if s.DefaultCurrency == "" {
		return NewError("TENANT_SETTINGS_INVALID", "default currency is required")
	}
	if err := validateTimezone(s.Timezone); err != nil {
		return err
	}
	if err := validateFeatureList(s.EnabledFeatures); err != nil {
		return err
	}
	if err := validateFeatureList(s.EnabledPaymentMethods); err != nil {
		return err
	}
	return nil
}

func validateTimezone(tz string) error {
	if strings.TrimSpace(tz) == "" {
		return NewError("TENANT_SETTINGS_INVALID", "timezone is required")
	}
	if len(tz) > 64 {
		return NewError("TENANT_SETTINGS_INVALID", "timezone is too long")
	}
	for _, r := range tz {
		if unicode.IsControl(r) {
			return NewError("TENANT_SETTINGS_INVALID", "timezone must not contain control characters")
		}
		if r == ' ' || r == '\t' || r == '\n' {
			return NewError("TENANT_SETTINGS_INVALID", "timezone must not contain whitespace")
		}
	}
	return nil
}

func validateFeatureList(items []string) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			return NewError("TENANT_SETTINGS_INVALID", "feature entries must be non-empty")
		}
		if len(trimmed) > 64 {
			return NewError("TENANT_SETTINGS_INVALID", "feature entry is too long")
		}
		if item != trimmed {
			return NewError("TENANT_SETTINGS_INVALID", "feature entry must not contain leading or trailing whitespace")
		}
		for _, r := range trimmed {
			if unicode.IsControl(r) {
				return NewError("TENANT_SETTINGS_INVALID", "feature entry must not contain control characters")
			}
		}
		lower := strings.ToLower(trimmed)
		if _, ok := seen[lower]; ok {
			return NewError("TENANT_SETTINGS_INVALID", "feature entries must be unique")
		}
		seen[lower] = struct{}{}
	}
	return nil
}

// TenantData is the persistence record for a tenant: identity, region,
// lifecycle, and settings. Name uniqueness is a repository concern; the
// domain checks the caller-supplied set deterministically in the onboarding
// service. It carries no balance and no secrets.
type TenantData struct {
	ID        valueobject.TenantID
	Name      string
	Alias     string
	Region    string
	Status    TenantStatus
	Settings  TenantSettings
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Validate checks structural scope and classification. An empty ID is permitted
// prior to persistence; if set, it must be a valid canonical UUID.
func (t TenantData) Validate() error {
	if t.ID.String() != "" {
		if _, err := valueobject.ParseTenantID(t.ID.String()); err != nil {
			return NewError("TENANT_ID_INVALID", "tenant id is invalid")
		}
	}
	if err := ValidateTenantName(t.Name); err != nil {
		return err
	}
	if err := ValidateTenantAlias(t.Alias); err != nil {
		return err
	}
	if err := validateTenantRegion(t.Region); err != nil {
		return err
	}
	if _, err := ParseTenantStatus(string(t.Status)); err != nil {
		return err
	}
	if err := t.Settings.Validate(); err != nil {
		return err
	}
	if t.Version < 1 {
		return NewError("TENANT_VERSION_INVALID", "version starts at 1")
	}
	if t.CreatedAt.IsZero() || t.UpdatedAt.IsZero() {
		return NewError("TENANT_TIME_REQUIRED", "tenant timestamps are required")
	}
	return nil
}

func validateTenantRegion(region string) error {
	trimmed := strings.TrimSpace(region)
	if trimmed == "" {
		return NewError("TENANT_REGION_REQUIRED", "tenant region is required")
	}
	if len(trimmed) > 32 {
		return NewError("TENANT_REGION_INVALID", "tenant region is too long")
	}
	if region != trimmed || strings.Contains(region, " ") {
		return NewError("TENANT_REGION_INVALID", "tenant region must not contain whitespace")
	}
	for _, r := range region {
		if unicode.IsControl(r) {
			return NewError("TENANT_REGION_INVALID", "tenant region must not contain control characters")
		}
	}
	return nil
}

// ValidateTenantName checks the 3–64 trimmed, control-free name shape.
func ValidateTenantName(name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return NewError("TENANT_NAME_REQUIRED", "tenant name is required")
	}
	if len(trimmed) < 3 {
		return NewError("TENANT_NAME_INVALID", "tenant name must be at least 3 characters")
	}
	if len(trimmed) > 64 {
		return NewError("TENANT_NAME_INVALID", "tenant name must be at most 64 characters")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return NewError("TENANT_NAME_INVALID", "tenant name must not contain control characters")
		}
	}
	return nil
}

// ValidateTenantAlias checks the URL-safe slug shape when an alias is set.
// Empty is allowed here (E06-T14 owns assignment at onboarding); the database
// NOT NULL + UNIQUE constraints reject missing or duplicate aliases at write
// time, so unset aliases fail closed at the boundary, never silently.
func ValidateTenantAlias(alias string) error {
	return ValidateAliasSlug(alias, "TENANT_ALIAS_INVALID", "tenant alias")
}

// isAliasChar reports whether r belongs in a URL slug.
func isAliasChar(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-'
}

// ValidateAliasSlug checks the URL-safe slug shape shared by tenant and
// ledger aliases: 3–64 lowercase alphanumeric/hyphen characters, never
// leading or trailing with a hyphen. Empty passes (assignment is a
// flow-level concern); set values must be well-formed.
func ValidateAliasSlug(alias, code, kind string) error {
	if alias == "" {
		return nil
	}

	if len(alias) < 3 || len(alias) > 64 {
		return NewError(code, kind+" must be 3–64 characters")
	}

	for _, r := range alias {
		if !isAliasChar(r) {
			return NewError(code, kind+" must be lowercase alphanumeric with hyphens")
		}
	}

	if alias[0] == '-' || alias[len(alias)-1] == '-' {
		return NewError(code, kind+" must not start or end with a hyphen")
	}

	return nil
}
