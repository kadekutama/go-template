package apikey

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/crc32"
	"slices"
	"strings"
	"time"

	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// APIKeyParams carries constructor dependencies (Parameter Object pattern).
// Hasher is required; pass an explicit NewHasher(HasherParams) configured for
// the deployment hardware.
type APIKeyParams struct {
	DB             *gorm.DB
	Hasher         Hasher
	RotationWindow time.Duration `validate:"gte=0"`
	Clock          appport.Clock
	OnUse          func(ctx context.Context, id string)
}

// apiKeyModel maps the api_keys table (migration 20260923000007).
// Hash/salt persist in BYTEA columns with a version tag: no PHC string is
// ever parsed (see Hasher). IDs are database-minted (uuidv7() default per
// ADR-019 / E06-T15): the adapter never pre-mints them.
type apiKeyModel struct {
	ID        string     `gorm:"column:id;type:uuid;primaryKey;default:uuidv7()"`
	TenantID  string     `gorm:"column:tenant_id"`
	Name      string     `gorm:"column:name"`
	Prefix    string     `gorm:"column:prefix;uniqueIndex"`
	Hash      []byte     `gorm:"column:hash"`
	Salt      []byte     `gorm:"column:salt"`
	Version   int        `gorm:"column:version"`
	Scopes    string     `gorm:"column:scopes"`
	ExpiresAt time.Time  `gorm:"column:expires_at"`
	Revoked   bool       `gorm:"column:revoked"`
	RotatesAt *time.Time `gorm:"column:rotates_at"`
	CreatedAt time.Time  `gorm:"column:created_at"`
}

// TableName pins the GORM model to the migrated table.
func (apiKeyModel) TableName() string { return "api_keys" }

// Manager mints, verifies, and revokes tenant API keys against PostgreSQL.
// Rows are the source of truth: a restart or a second replica changes
// nothing. Concurrency control is the prefix unique index plus row writes;
// the adapter holds no locks and no maps.
type Manager struct {
	db             *gorm.DB
	rotationWindow time.Duration
	clock          appport.Clock
	hasher         Hasher
	onUse          func(ctx context.Context, id string)
}

// Compile-time port conformance.
var _ appport.APIKeyStore = (*Manager)(nil)

// NewManager builds the API key adapter; DB, Clock, and Hasher must be
// non-nil and RotationWindow must be positive (a zero window would leave
// rotated keys valid indefinitely — fail fast instead of defaulting).
func NewManager(params APIKeyParams) (*Manager, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("%w: DB is required", ErrConfigRequired)
	}

	if params.Clock == nil {
		return nil, ErrClockRequired
	}

	if params.Hasher.IsZero() {
		return nil, fmt.Errorf("%w: Hasher is required", ErrConfigRequired)
	}

	if params.RotationWindow <= 0 {
		return nil, fmt.Errorf("%w: rotation window must be positive", ErrConfigRequired)
	}

	return &Manager{
		db:             params.DB,
		hasher:         params.Hasher,
		rotationWindow: params.RotationWindow,
		clock:          params.Clock,
		onUse:          params.OnUse,
	}, nil
}

// now returns the current UTC time through the injected clock.
func (m *Manager) now() time.Time {
	return m.clock.Now().UTC()
}

// Create mints one key with its single plaintext reveal.
func (m *Manager) Create(ctx context.Context, tenant valueobject.TenantID, name string, scopes []string, ttl time.Duration) (appport.APIKeySecret, error) {
	if m == nil || m.db == nil || m.clock == nil {
		return appport.APIKeySecret{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return appport.APIKeySecret{}, fmt.Errorf("apikey: create: %w", err)
	}

	if tenant == "" || name == "" || ttl <= 0 {
		return appport.APIKeySecret{}, fmt.Errorf("%w: tenant, name and positive ttl required", ErrConfigRequired)
	}

	if _, err := valueobject.ParseTenantID(string(tenant)); err != nil {
		return appport.APIKeySecret{}, fmt.Errorf("%w: tenant: %s", ErrConfigRequired, err.Error())
	}

	row, plaintext, err := m.mintRow(tenant, name, scopes, ttl)
	if err != nil {
		return appport.APIKeySecret{}, err
	}

	if err := postgres.WithinTenantTx(ctx, m.db, tenant, func(tx *gorm.DB) error {
		return tx.Create(&row).Error
	}); err != nil {
		return appport.APIKeySecret{}, err
	}

	return appport.APIKeySecret{
		Key: appport.APIKey{
			ID:        row.ID,
			TenantID:  tenant,
			Name:      name,
			Prefix:    row.Prefix,
			Scopes:    append([]string(nil), scopes...),
			ExpiresAt: row.ExpiresAt,
		},
		Plaintext: plaintext,
	}, nil
}

// mintRow formats one key, seals it, and builds the row without touching
// the database. ID stays empty: PostgreSQL mints uuidv7() on insert and
// GORM reads it back (E06-T15, no app pre-mint).
func (m *Manager) mintRow(tenant valueobject.TenantID, name string, scopes []string, ttl time.Duration) (apiKeyModel, string, error) {
	prefix := newPrefix()
	secret := newSecret()
	plaintext := "ak_" + prefix + "." + string(tenant) + "." + secret + "." + checksumOf(prefix, string(tenant), secret)

	sealed, err := m.hasher.Hash(plaintext)
	if err != nil {
		return apiKeyModel{}, "", err
	}

	scopesJSON, err := jsonparser.Marshal(append([]string(nil), scopes...))
	if err != nil {
		return apiKeyModel{}, "", err
	}

	return apiKeyModel{
		TenantID:  string(tenant),
		Name:      name,
		Prefix:    prefix,
		Hash:      sealed.Hash,
		Salt:      sealed.Salt,
		Version:   sealed.Version,
		Scopes:    string(scopesJSON),
		ExpiresAt: m.now().Add(ttl),
	}, plaintext, nil
}

// Verify authenticates one plaintext key and fails closed. Lookup,
// validity, hash verification, and the transparent re-hash upgrade all run
// inside one tenant-scoped transaction: the RLS GUC is set (CR-001), and a
// failed upgrade rolls the whole verification back instead of silently
// degrading to a no-op (CR-009).
func (m *Manager) Verify(ctx context.Context, plaintext string) (appport.APIKey, error) {
	prefix, tenant, err := m.parseVerifyInput(ctx, plaintext)
	if err != nil {
		return appport.APIKey{}, err
	}

	var row apiKeyModel

	if err := postgres.WithinTenantTx(ctx, m.db, tenant, func(tx *gorm.DB) error {
		loaded, err := m.loadRow(tx, prefix, tenant)
		if err != nil {
			return err
		}

		upgraded, err := m.verifyRow(tx, loaded, plaintext)
		if err != nil {
			return err
		}

		row = upgraded

		return nil
	}); err != nil {
		return appport.APIKey{}, err
	}

	if m.onUse != nil {
		m.onUse(ctx, row.ID)
	}

	return m.toPort(row)
}

// parseVerifyInput guards the receiver/context and parses the tenant-scoped
// lookup key out of the plaintext.
func (m *Manager) parseVerifyInput(ctx context.Context, plaintext string) (string, valueobject.TenantID, error) {
	if m == nil || m.db == nil || m.clock == nil {
		return "", "", ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return "", "", fmt.Errorf("apikey: verify: %w", err)
	}

	prefix, tenantHint, secret, checksum, err := splitPlaintext(plaintext)
	if err != nil {
		return "", "", err
	}

	if checksumOf(prefix, tenantHint, secret) != checksum {
		return "", "", ErrInvalidFormat
	}

	tenant, err := valueobject.ParseTenantID(tenantHint)
	if err != nil {
		return "", "", ErrInvalidFormat
	}

	return prefix, tenant, nil
}

// loadRow loads one key row by prefix inside the caller's tenant-scoped
// transaction (GUC already set by the caller).
func (m *Manager) loadRow(tx *gorm.DB, prefix string, tenant valueobject.TenantID) (apiKeyModel, error) {
	var row apiKeyModel

	if err := tx.Where("prefix = ? AND tenant_id = ?", prefix, string(tenant)).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apiKeyModel{}, ErrNotFound
		}

		return apiKeyModel{}, fmt.Errorf("apikey: verify lookup: %w", err)
	}

	return row, nil
}

// verifyRow checks validity + hash and persists the transparent re-hash
// upgrade inside the caller's transaction.
func (m *Manager) verifyRow(tx *gorm.DB, row apiKeyModel, plaintext string) (apiKeyModel, error) {
	if err := m.checkValidity(row); err != nil {
		return apiKeyModel{}, err
	}

	valid, needsUpgrade := m.hasher.VerifyWithUpgrade(Sealed{Hash: row.Hash, Salt: row.Salt, Version: row.Version}, plaintext)
	if !valid {
		return apiKeyModel{}, ErrNotFound
	}

	if !needsUpgrade {
		return row, nil
	}

	// Transparently re-hash and persist the active version/parameters.
	// Any failure aborts verification: a half-upgraded row must never
	// commit, and crypto failures fail closed on an auth path.
	newSealed, err := m.hasher.Hash(plaintext)
	if err != nil {
		return apiKeyModel{}, fmt.Errorf("apikey: verify re-hash: %w", err)
	}

	if err := tx.Model(&apiKeyModel{}).
		Where("id = ? AND tenant_id = ?", row.ID, row.TenantID).
		Updates(map[string]any{
			"hash":    newSealed.Hash,
			"salt":    newSealed.Salt,
			"version": newSealed.Version,
		}).Error; err != nil {
		return apiKeyModel{}, fmt.Errorf("apikey: verify upgrade: %w", err)
	}

	row.Hash = newSealed.Hash
	row.Salt = newSealed.Salt
	row.Version = newSealed.Version

	return row, nil
}

// Revoke disables one key by ID and is idempotent.
func (m *Manager) Revoke(ctx context.Context, tenant valueobject.TenantID, id string) error {
	if m == nil || m.db == nil || m.clock == nil {
		return ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("apikey: revoke: %w", err)
	}

	return postgres.WithinTenantTx(ctx, m.db, tenant, func(tx *gorm.DB) error {
		result := tx.Model(&apiKeyModel{}).
			Where("id = ? AND tenant_id = ?", id, string(tenant)).
			Update("revoked", true)
		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return ErrNotFound
		}

		return nil
	})
}

// Rotate mints a successor for id, keeping the old key valid for the
// rotation window (dual-active). Mint, insert, and the old key's rotates_at
// stamp commit in one tenant-scoped transaction: a crash between them can no
// longer leave a live successor with an unstamped predecessor (CR-008).
// Adapter-specific; port stays exact.
func (m *Manager) Rotate(ctx context.Context, tenant valueobject.TenantID, id string, ttl time.Duration) (appport.APIKeySecret, error) {
	if err := m.guardRotateInput(ctx, tenant, ttl); err != nil {
		return appport.APIKeySecret{}, err
	}

	var secret appport.APIKeySecret

	if err := postgres.WithinTenantTx(ctx, m.db, tenant, func(tx *gorm.DB) error {
		minted, err := m.mintSuccessor(tx, tenant, id, ttl)
		if err != nil {
			return err
		}

		secret = minted

		return nil
	}); err != nil {
		return appport.APIKeySecret{}, err
	}

	return secret, nil
}

// guardRotateInput validates receiver, context, and rotation arguments.
func (m *Manager) guardRotateInput(ctx context.Context, tenant valueobject.TenantID, ttl time.Duration) error {
	if m == nil || m.db == nil || m.clock == nil {
		return ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("apikey: rotate: %w", err)
	}

	if tenant == "" || ttl <= 0 {
		return fmt.Errorf("%w: tenant and positive ttl required", ErrConfigRequired)
	}

	return nil
}

// mintSuccessor loads the predecessor, mints and inserts the successor, and
// stamps rotates_at — all inside the caller's tenant-scoped transaction.
func (m *Manager) mintSuccessor(tx *gorm.DB, tenant valueobject.TenantID, id string, ttl time.Duration) (appport.APIKeySecret, error) {
	var old apiKeyModel

	if err := tx.Where("id = ? AND tenant_id = ?", id, string(tenant)).First(&old).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return appport.APIKeySecret{}, ErrNotFound
		}

		return appport.APIKeySecret{}, fmt.Errorf("apikey: rotate load: %w", err)
	}

	oldPort, err := m.toPort(old)
	if err != nil {
		return appport.APIKeySecret{}, err
	}

	newRow, plaintext, err := m.mintRow(tenant, oldPort.Name, oldPort.Scopes, ttl)
	if err != nil {
		return appport.APIKeySecret{}, err
	}

	if err := tx.Create(&newRow).Error; err != nil {
		return appport.APIKeySecret{}, fmt.Errorf("apikey: rotate mint: %w", err)
	}

	if err := tx.Model(&apiKeyModel{}).
		Where("id = ? AND tenant_id = ?", id, string(tenant)).
		Update("rotates_at", m.now().Add(m.rotationWindow)).Error; err != nil {
		return appport.APIKeySecret{}, fmt.Errorf("apikey: rotate stamp: %w", err)
	}

	return appport.APIKeySecret{
		Key: appport.APIKey{
			ID:        newRow.ID,
			TenantID:  tenant,
			Name:      newRow.Name,
			Prefix:    newRow.Prefix,
			Scopes:    append([]string(nil), oldPort.Scopes...),
			ExpiresAt: newRow.ExpiresAt,
		},
		Plaintext: plaintext,
	}, nil
}

// checkValidity enforces revocation, expiry, and rotation windows.
func (m *Manager) checkValidity(row apiKeyModel) error {
	if row.Revoked {
		return ErrRevoked
	}

	if m.now().After(row.ExpiresAt) {
		return ErrExpired
	}

	if row.RotatesAt != nil && m.now().After(*row.RotatesAt) {
		return ErrRevoked
	}

	return nil
}

// toPort maps one row onto the port shape, decoding scopes.
func (m *Manager) toPort(row apiKeyModel) (appport.APIKey, error) {
	var scopes []string

	if err := jsonparser.Unmarshal([]byte(row.Scopes), &scopes); err != nil {
		return appport.APIKey{}, ErrNotFound
	}

	return appport.APIKey{
		ID:        row.ID,
		TenantID:  valueobject.TenantID(row.TenantID),
		Name:      row.Name,
		Prefix:    row.Prefix,
		Scopes:    scopes,
		ExpiresAt: row.ExpiresAt,
		Revoked:   row.Revoked,
	}, nil
}

// HasScope reports whether key carries scope (edge maps false to FORBIDDEN).
func HasScope(key appport.APIKey, scope string) bool {
	return slices.Contains(key.Scopes, scope)
}

// newPrefix returns one random 8-char prefix.
func newPrefix() string {
	var buf [6]byte

	_, _ = rand.Read(buf[:])

	return base64.RawURLEncoding.EncodeToString(buf[:])
}

// newSecret returns one random 32-byte secret (256-bit entropy).
func newSecret() string {
	var buf [32]byte

	_, _ = rand.Read(buf[:])

	return base64.RawURLEncoding.EncodeToString(buf[:])
}

// checksumOf binds prefix + tenant + secret (format integrity, not secrecy).
func checksumOf(prefix, tenant, secret string) string {
	sum := crc32.ChecksumIEEE([]byte(prefix + "." + tenant + "." + secret))

	return fmt.Sprintf("%08x", sum)
}

// splitPlaintext parses ak_PREFIX.TENANT.SECRET.CHECKSUM. The tenant segment
// lets Verify scope RLS before the row is known; it is re-checked against
// the stored row implicitly by the lookup predicate.
func splitPlaintext(plaintext string) (prefix, tenant, secret, checksum string, err error) {
	rest, ok := strings.CutPrefix(plaintext, "ak_")
	if !ok {
		return "", "", "", "", ErrInvalidFormat
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return "", "", "", "", ErrInvalidFormat
	}

	return parts[0], parts[1], parts[2], parts[3], nil
}
