package service

import (
	"slices"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/event"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Default tenant purposes for the initial chart of accounts.
const (
	TenantPurposeOperating = "operating"
	TenantPurposeFee       = "fee"
	TenantPurposeSuspense  = "suspense"
)

// MaxOnboardingAssets bounds the per-tenant default chart expansion.
const MaxOnboardingAssets = 8

// OnboardingRequest carries the self-serve provisioning command. ExistingNames
// and AllowedRegions are caller-supplied authority (repository/config); the
// domain never reads global state.
type OnboardingRequest struct {
	TenantID       valueobject.TenantID
	LedgerID       valueobject.LedgerID
	Name           string
	Region         string
	Settings       entity.TenantSettings
	Assets         []valueobject.AssetCode
	ExistingNames  []string
	AllowedRegions []string
	RequestedBy    valueobject.UserID
	EventID        string
	Now            time.Time
}

// DefaultAccount is one deterministic initial chart row.
type DefaultAccount struct {
	Purpose   string
	AssetCode valueobject.AssetCode
	Number    string
	Name      string
	Class     valueobject.AccountClass
}

// APIKeyDescriptor is the domain-visible key identity. Secret material is
// issued by E09; the domain carries IDs/prefix only.
type APIKeyDescriptor struct {
	KeyID     string
	KeyPrefix string
	TenantID  string
	IssuedAt  time.Time
}

// OnboardingPlan is the all-or-nothing provisioning outcome.
type OnboardingPlan struct {
	Tenant   entity.TenantData
	LedgerID string
	Accounts []DefaultAccount
	Key      APIKeyDescriptor
	Event    event.TenantCreatedPayload
}

// IsRegionAllowed reports whether region is in the allowlist
// (case-insensitive, trimmed). Empty inputs never pass.
func IsRegionAllowed(region string, allowed []string) bool {
	trimmed := strings.TrimSpace(region)
	if trimmed == "" || len(allowed) == 0 {
		return false
	}
	for _, a := range allowed {
		if strings.EqualFold(strings.TrimSpace(a), trimmed) {
			return true
		}
	}
	return false
}

// IsNameTaken reports case-insensitive trimmed name collision.
func IsNameTaken(name string, existing []string) bool {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return false
	}
	for _, e := range existing {
		if strings.EqualFold(strings.TrimSpace(e), trimmed) {
			return true
		}
	}
	return false
}

// DefaultAccountsForAssets expands operating + fee + suspense per asset with
// deterministic numbers, names, and classes.
func DefaultAccountsForAssets(assets []valueobject.AssetCode) ([]DefaultAccount, error) {
	if len(assets) == 0 {
		return nil, entity.NewError("ONBOARDING_ASSETS_REQUIRED", "onboarding requires at least one asset")
	}
	if len(assets) > MaxOnboardingAssets {
		return nil, entity.NewError("ONBOARDING_ASSETS_INVALID", "onboarding requests too many assets")
	}
	seen := make(map[valueobject.AssetCode]struct{}, len(assets))
	out := make([]DefaultAccount, 0, len(assets)*3)
	for _, asset := range assets {
		trimmed := strings.TrimSpace(string(asset))
		if trimmed == "" {
			return nil, entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding asset code is required")
		}
		cleanAsset := valueobject.AssetCode(trimmed)
		if _, ok := seen[cleanAsset]; ok {
			return nil, entity.NewError("ONBOARDING_ASSET_INVALID", "onboarding assets must be unique")
		}
		seen[cleanAsset] = struct{}{}
		out = append(out,
			DefaultAccount{Purpose: TenantPurposeOperating, AssetCode: cleanAsset, Number: "1000-" + string(cleanAsset), Name: "Operating " + string(cleanAsset), Class: valueobject.ClassLiability},
			DefaultAccount{Purpose: TenantPurposeFee, AssetCode: cleanAsset, Number: "4000-" + string(cleanAsset), Name: "Platform Fees " + string(cleanAsset), Class: valueobject.ClassRevenue},
			DefaultAccount{Purpose: TenantPurposeSuspense, AssetCode: cleanAsset, Number: "1100-" + string(cleanAsset), Name: "Suspense " + string(cleanAsset), Class: valueobject.ClassAsset},
		)
	}
	return out, nil
}

// ValidateOnboarding checks the full checklist and returns a complete plan or
// an error with no partial plan.
func ValidateOnboarding(req OnboardingRequest) (OnboardingPlan, error) {
	if err := validateOnboardingIdentity(req); err != nil {
		return OnboardingPlan{}, err
	}
	if err := validateOnboardingPolicy(req); err != nil {
		return OnboardingPlan{}, err
	}
	if err := validateOnboardingEnvelope(req); err != nil {
		return OnboardingPlan{}, err
	}
	accounts, err := DefaultAccountsForAssets(req.Assets)
	if err != nil {
		return OnboardingPlan{}, err
	}
	hasDefaultCurrency := false
	for _, a := range req.Assets {
		if strings.TrimSpace(string(a)) == string(req.Settings.DefaultCurrency) {
			hasDefaultCurrency = true
			break
		}
	}
	if !hasDefaultCurrency {
		return OnboardingPlan{}, entity.NewError("ONBOARDING_ASSET_INVALID", "default currency must be included in onboarding assets")
	}
	settings := req.Settings
	settings.EnabledFeatures = slices.Clone(req.Settings.EnabledFeatures)
	settings.EnabledPaymentMethods = slices.Clone(req.Settings.EnabledPaymentMethods)
	tenant := entity.TenantData{
		ID:        req.TenantID,
		Name:      strings.TrimSpace(req.Name),
		Region:    strings.TrimSpace(req.Region),
		Status:    entity.TenantActive,
		Settings:  settings,
		Version:   1,
		CreatedAt: req.Now.UTC(),
		UpdatedAt: req.Now.UTC(),
	}
	if err := tenant.Validate(); err != nil {
		return OnboardingPlan{}, err
	}
	tenantID := req.TenantID.String()
	prefix := tenantID
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	key := APIKeyDescriptor{
		KeyID:     "key_" + tenantID,
		KeyPrefix: "pk_" + prefix,
		TenantID:  tenantID,
		IssuedAt:  req.Now.UTC(),
	}
	payload := event.TenantCreatedPayload{
		TenantID:      tenantID,
		LedgerID:      req.LedgerID.String(),
		Name:          tenant.Name,
		Region:        tenant.Region,
		BaseAssetCode: string(req.Settings.DefaultCurrency),
		CreatedAt:     req.Now.UTC(),
	}
	return OnboardingPlan{
		Tenant:   tenant,
		LedgerID: req.LedgerID.String(),
		Accounts: accounts,
		Key:      key,
		Event:    payload,
	}, nil
}

func validateOnboardingIdentity(req OnboardingRequest) error {
	if strings.TrimSpace(req.TenantID.String()) == "" {
		return entity.NewError("TENANT_ID_REQUIRED", "tenant id is required")
	}
	if strings.TrimSpace(req.LedgerID.String()) == "" {
		return entity.NewError("LEDGER_REQUIRED", "ledger id is required")
	}
	if err := entity.ValidateTenantName(req.Name); err != nil {
		return err
	}
	if strings.TrimSpace(req.Region) == "" {
		return entity.NewError("TENANT_REGION_REQUIRED", "tenant region is required")
	}
	return nil
}

func validateOnboardingPolicy(req OnboardingRequest) error {
	if !IsRegionAllowed(req.Region, req.AllowedRegions) {
		return entity.NewError("TENANT_REGION_INVALID", "tenant region is not allowed")
	}
	if IsNameTaken(req.Name, req.ExistingNames) {
		return entity.NewError("TENANT_NAME_DUPLICATE", "tenant name is already taken")
	}
	if err := req.Settings.Validate(); err != nil {
		return err
	}
	return nil
}

func validateOnboardingEnvelope(req OnboardingRequest) error {
	if strings.TrimSpace(req.RequestedBy.String()) == "" {
		return entity.NewError("ONBOARDING_ACTOR_REQUIRED", "onboarding actor is required")
	}
	if strings.TrimSpace(req.EventID) == "" {
		return entity.NewError("ONBOARDING_EVENT_REQUIRED", "onboarding event id is required")
	}
	if req.Now.IsZero() {
		return entity.NewError("ONBOARDING_TIME_REQUIRED", "onboarding time is required")
	}
	return nil
}
