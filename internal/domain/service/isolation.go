package service

import (
	"strings"
	"unicode"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// Isolation key prefixes. Cache keys are hints only; the durable database key
// is scoped by the same tenant/operation/key tuple (ledger-core §8).
const (
	prefixBalance     = "balance:"
	prefixIdempotency = "idempotency:"
	prefixRateLimit   = "ratelimit:"
	prefixSubject     = "ledger."
)

// maxIsolationSegment bounds one key segment against abuse.
const maxIsolationSegment = 128

func validateKeySegment(segment string) error {
	trimmed := strings.TrimSpace(segment)
	if trimmed == "" {
		return entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is required")
	}
	if len(trimmed) > maxIsolationSegment {
		return entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment is too long")
	}
	if strings.Contains(trimmed, ":") {
		return entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain ':'")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return entity.NewError("ISOLATION_KEY_INVALID", "isolation key segment must not contain control characters")
		}
	}
	return nil
}

// BuildBalanceKey returns balance:{tenant}:{account}:{asset}.
func BuildBalanceKey(tenant, account, asset string) (string, error) {
	for _, segment := range []string{tenant, account, asset} {
		if err := validateKeySegment(segment); err != nil {
			return "", err
		}
	}
	return prefixBalance + strings.TrimSpace(tenant) + ":" + strings.TrimSpace(account) + ":" + strings.TrimSpace(asset), nil
}

// BuildBalanceCursorKey returns balance:{tenant}:{account}:{asset}:{cursor}.
func BuildBalanceCursorKey(tenant, account, asset, cursor string) (string, error) {
	for _, segment := range []string{tenant, account, asset, cursor} {
		if err := validateKeySegment(segment); err != nil {
			return "", err
		}
	}
	key, err := BuildBalanceKey(tenant, account, asset)
	if err != nil {
		return "", err
	}
	return key + ":" + strings.TrimSpace(cursor), nil
}

// ParseBalanceKey inverts BuildBalanceKey/BuildBalanceCursorKey. Three segments
// yield an empty cursor; four segments yield the cursor.
func ParseBalanceKey(key string) (tenant, account, asset, cursor string, err error) {
	rest, ok := strings.CutPrefix(key, prefixBalance)
	if !ok {
		return "", "", "", "", entity.NewError("ISOLATION_KEY_INVALID", "balance key must start with 'balance:'")
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 && len(parts) != 4 {
		return "", "", "", "", entity.NewError("ISOLATION_KEY_INVALID", "balance key must have 3 or 4 segments")
	}
	for _, segment := range parts {
		if err := validateKeySegment(segment); err != nil {
			return "", "", "", "", err
		}
	}
	if len(parts) == 3 {
		return parts[0], parts[1], parts[2], "", nil
	}
	return parts[0], parts[1], parts[2], parts[3], nil
}

// BuildIdempotencyKey returns idempotency:{tenant}:{operation}:{key}.
func BuildIdempotencyKey(tenant, operation, key string) (string, error) {
	for _, segment := range []string{tenant, operation, key} {
		if err := validateKeySegment(segment); err != nil {
			return "", err
		}
	}
	return prefixIdempotency + strings.TrimSpace(tenant) + ":" + strings.TrimSpace(operation) + ":" + strings.TrimSpace(key), nil
}

// ParseIdempotencyKey inverts BuildIdempotencyKey.
func ParseIdempotencyKey(key string) (tenant, operation, idempotencyKey string, err error) {
	rest, ok := strings.CutPrefix(key, prefixIdempotency)
	if !ok {
		return "", "", "", entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must start with 'idempotency:'")
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 3 {
		return "", "", "", entity.NewError("ISOLATION_KEY_INVALID", "idempotency key must have 3 segments")
	}
	for _, segment := range parts {
		if err := validateKeySegment(segment); err != nil {
			return "", "", "", err
		}
	}
	return parts[0], parts[1], parts[2], nil
}

// BuildRateLimitKey returns ratelimit:{tenant}:{user}.
func BuildRateLimitKey(tenant, user string) (string, error) {
	for _, segment := range []string{tenant, user} {
		if err := validateKeySegment(segment); err != nil {
			return "", err
		}
	}
	return prefixRateLimit + strings.TrimSpace(tenant) + ":" + strings.TrimSpace(user), nil
}

// ParseRateLimitKey inverts BuildRateLimitKey.
func ParseRateLimitKey(key string) (tenant, user string, err error) {
	rest, ok := strings.CutPrefix(key, prefixRateLimit)
	if !ok {
		return "", "", entity.NewError("ISOLATION_KEY_INVALID", "rate-limit key must start with 'ratelimit:'")
	}
	parts := strings.Split(rest, ":")
	if len(parts) != 2 {
		return "", "", entity.NewError("ISOLATION_KEY_INVALID", "rate-limit key must have 2 segments")
	}
	for _, segment := range parts {
		if err := validateKeySegment(segment); err != nil {
			return "", "", err
		}
	}
	return parts[0], parts[1], nil
}

// BuildSubject returns ledger.{tenant}.{event_type}. The event type already
// contains domain/action segments and is never re-scoped.
func BuildSubject(tenant, eventType string) (string, error) {
	if err := validateSubjectTenant(tenant); err != nil {
		return "", err
	}
	if err := validateSubjectEventType(eventType); err != nil {
		return "", err
	}
	return prefixSubject + strings.TrimSpace(tenant) + "." + strings.TrimSpace(eventType), nil
}

// ParseSubject inverts BuildSubject.
func ParseSubject(subject string) (tenant, eventType string, err error) {
	rest, ok := strings.CutPrefix(subject, prefixSubject)
	if !ok {
		return "", "", entity.NewError("ISOLATION_SUBJECT_INVALID", "subject must start with 'ledger.'")
	}
	dot := strings.Index(rest, ".")
	if dot <= 0 || dot >= len(rest)-1 {
		return "", "", entity.NewError("ISOLATION_SUBJECT_INVALID", "subject must carry tenant and event type")
	}
	tenantPart := rest[:dot]
	eventPart := rest[dot+1:]
	if err := validateSubjectTenant(tenantPart); err != nil {
		return "", "", err
	}
	if err := validateSubjectEventType(eventPart); err != nil {
		return "", "", err
	}
	return tenantPart, eventPart, nil
}

func validateSubjectTenant(tenant string) error {
	trimmed := strings.TrimSpace(tenant)
	if trimmed == "" {
		return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is required")
	}
	if len(trimmed) > 64 {
		return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant is too long")
	}
	if strings.Contains(trimmed, ".") || strings.Contains(trimmed, ":") {
		return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant must not contain '.' or ':'")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject tenant must not contain whitespace")
		}
	}
	return nil
}

func validateSubjectEventType(eventType string) error {
	trimmed := strings.TrimSpace(eventType)
	if trimmed == "" {
		return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type is required")
	}
	if !strings.Contains(trimmed, ".") {
		return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type must be versioned")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return entity.NewError("ISOLATION_SUBJECT_INVALID", "subject event type must not contain whitespace")
		}
	}
	return nil
}

// RequireTenant fails closed on missing tenant scope.
func RequireTenant(tenant string) error {
	if strings.TrimSpace(tenant) == "" {
		return entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}
	return nil
}

// RLSPolicy is one tenant×table×operation contract row for E07-T02.
type RLSPolicy struct {
	Table     string
	Operation string
	Scope     string
}

// RLS scopes.
const (
	RLSScopeTenant     = "tenant"
	RLSScopeSelf       = "self"
	RLSScopeSharedRead = "shared-read"
	RLSScopeService    = "service"
)

// RLSPolicyMatrix covers every data-flow §5 core table. Global tables are
// shared-read; tenant tables are tenant-scoped; the tenants table itself is
// self-scoped; inbox receipts are service-scoped (no tenant column).
func RLSPolicyMatrix() []RLSPolicy {
	tables := []struct {
		name  string
		scope string
	}{
		{"tenants", RLSScopeSelf},
		{"asset_registry", RLSScopeSharedRead},
		{"ledgers", RLSScopeTenant},
		{"accounts", RLSScopeTenant},
		{"postings", RLSScopeTenant},
		{"entries", RLSScopeTenant},
		{"balance_checkpoints", RLSScopeTenant},
		{"holds", RLSScopeTenant},
		{"idempotency_records", RLSScopeTenant},
		{"outbox_events", RLSScopeTenant},
		{"inbox_receipts", RLSScopeService},
	}
	operations := []string{"SELECT", "INSERT", "UPDATE", "DELETE"}
	out := make([]RLSPolicy, 0, len(tables)*len(operations))
	for _, table := range tables {
		for _, operation := range operations {
			scope := table.scope
			if table.name == "asset_registry" && operation != "SELECT" {
				scope = RLSScopeService
			}
			out = append(out, RLSPolicy{Table: table.name, Operation: operation, Scope: scope})
		}
	}
	return out
}

// ResidencySettings is the per-tenant region/DB pointer shape enforced in E07.
type ResidencySettings struct {
	Region    string
	DBPointer string
}

// Validate checks residency shape.
func (s ResidencySettings) Validate() error {
	if strings.TrimSpace(s.Region) == "" {
		return entity.NewError("RESIDENCY_REGION_REQUIRED", "residency region is required")
	}
	if len(strings.TrimSpace(s.Region)) > 32 {
		return entity.NewError("RESIDENCY_REGION_INVALID", "residency region is too long")
	}
	if strings.TrimSpace(s.DBPointer) == "" {
		return entity.NewError("RESIDENCY_DB_REQUIRED", "residency db pointer is required")
	}
	if len(s.DBPointer) > 256 {
		return entity.NewError("RESIDENCY_DB_INVALID", "residency db pointer is too long")
	}
	for _, value := range []string{s.Region, s.DBPointer} {
		for _, r := range value {
			if unicode.IsControl(r) {
				return entity.NewError("RESIDENCY_INVALID", "residency settings must not contain control characters")
			}
		}
	}
	for _, r := range s.DBPointer {
		if unicode.IsSpace(r) {
			return entity.NewError("RESIDENCY_DB_INVALID", "residency db pointer must not contain spaces")
		}
	}
	return nil
}

// WhiteLabelSettings is the branding/domain shape (validated format, no behavior).
type WhiteLabelSettings struct {
	BrandName    string
	Domains      []string
	LogoURL      string
	PrimaryColor string
}

// Validate checks white-label format.
func (s WhiteLabelSettings) Validate() error {
	trimmed := strings.TrimSpace(s.BrandName)
	if trimmed == "" {
		return entity.NewError("WHITELABEL_BRAND_REQUIRED", "brand name is required")
	}
	if len(trimmed) > 64 {
		return entity.NewError("WHITELABEL_BRAND_INVALID", "brand name is too long")
	}
	for _, r := range trimmed {
		if unicode.IsControl(r) {
			return entity.NewError("WHITELABEL_BRAND_INVALID", "brand name must not contain control characters")
		}
	}
	for _, domain := range s.Domains {
		if err := validateWhiteLabelDomain(domain); err != nil {
			return err
		}
	}
	if s.LogoURL != "" {
		if !strings.HasPrefix(s.LogoURL, "https://") || strings.Contains(s.LogoURL, " ") {
			return entity.NewError("WHITELABEL_LOGO_INVALID", "logo url must be an https url without spaces")
		}
	}
	if s.PrimaryColor != "" {
		if err := validateHexColor(s.PrimaryColor); err != nil {
			return err
		}
	}
	return nil
}

func validateWhiteLabelDomain(domain string) error {
	trimmed := strings.TrimSpace(domain)
	if err := checkWhiteLabelHostnameShape(trimmed); err != nil {
		return err
	}
	for _, label := range strings.Split(trimmed, ".") {
		if err := checkWhiteLabelLabel(label); err != nil {
			return err
		}
	}
	return nil
}

func checkWhiteLabelHostnameShape(trimmed string) error {
	if trimmed == "" {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain is required")
	}
	if len(trimmed) > 253 {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain is too long")
	}
	if strings.Contains(trimmed, "://") || strings.Contains(trimmed, "/") || strings.Contains(trimmed, " ") {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain must be a bare hostname")
	}
	if !strings.Contains(trimmed, ".") {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain must be fully qualified")
	}
	return nil
}

func checkWhiteLabelLabel(label string) error {
	if len(label) == 0 || len(label) > 63 {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain label is invalid")
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain label must not start or end with '-'")
	}
	for _, r := range label {
		if !isHostnameRune(r) {
			return entity.NewError("WHITELABEL_DOMAIN_INVALID", "white-label domain contains an illegal character")
		}
	}
	return nil
}

func isHostnameRune(r rune) bool {
	isLower := r >= 'a' && r <= 'z'
	isUpper := r >= 'A' && r <= 'Z'
	isDigit := r >= '0' && r <= '9'
	return isLower || isUpper || isDigit || r == '-'
}

func validateHexColor(color string) error {
	if !strings.HasPrefix(color, "#") {
		return entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be hex")
	}
	hex := strings.TrimPrefix(color, "#")
	if len(hex) != 3 && len(hex) != 6 {
		return entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be #RGB or #RRGGBB")
	}
	for _, r := range hex {
		isDigit := r >= '0' && r <= '9'
		isLower := r >= 'a' && r <= 'f'
		isUpper := r >= 'A' && r <= 'F'
		if !isDigit && !isLower && !isUpper {
			return entity.NewError("WHITELABEL_COLOR_INVALID", "primary color must be hex")
		}
	}
	return nil
}
