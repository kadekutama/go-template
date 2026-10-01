package jwt

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
)

// RefreshClaim is the result of spending one refresh token.
type RefreshClaim struct {
	FamilyID  string
	Subject   string
	TenantID  string
	IssuedAt  time.Time
	ExpiresAt time.Time
	NextToken string
}

// refreshFamilyModel maps refresh_families (migration 20260923000006).
type refreshFamilyModel struct {
	FamilyID string    `gorm:"column:family_id;primaryKey"`
	TenantID string    `gorm:"column:tenant_id"`
	Subject  string    `gorm:"column:subject"`
	Revoked  bool      `gorm:"column:revoked"`
	IssuedAt time.Time `gorm:"column:issued_at"`
}

// TableName pins the model to the migrated table.
func (refreshFamilyModel) TableName() string { return "refresh_families" }

// refreshTokenModel maps refresh_tokens. Only hashes persist; opaque tokens
// never touch the database.
type refreshTokenModel struct {
	TokenHash string `gorm:"column:token_hash;primaryKey"`
	FamilyID  string `gorm:"column:family_id"`
	TenantID  string `gorm:"column:tenant_id"`
	Used      bool   `gorm:"column:used"`
}

// TableName pins the model to the migrated table.
func (refreshTokenModel) TableName() string { return "refresh_tokens" }

// ReceiptStore tracks single-use refresh tokens with family revocation in
// PostgreSQL. The database is the source of truth: restarts and replicas
// change nothing. Concurrency control is row locks plus the token-hash
// primary key; the adapter holds no locks and no maps.
type ReceiptStore struct {
	db  *gorm.DB
	ttl time.Duration
	clk appport.Clock
}

// ReceiptParams carries constructor dependencies.
type ReceiptParams struct {
	DB    *gorm.DB
	TTL   time.Duration `validate:"required,gt=0"`
	Clock appport.Clock
}

// NewRefreshStore builds the receipt tracker; DB and Clock must be non-nil.
func NewRefreshStore(params ReceiptParams) (*ReceiptStore, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("%w: DB is required", ErrConfigRequired)
	}

	if params.Clock == nil {
		return nil, ErrClockRequired
	}

	if params.TTL <= 0 {
		return nil, fmt.Errorf("%w: positive TTL required", ErrConfigRequired)
	}

	return &ReceiptStore{db: params.DB, ttl: params.TTL, clk: params.Clock}, nil
}

// now returns the current UTC time through the injected clock.
func (s *ReceiptStore) now() time.Time {
	return s.clk.Now().UTC()
}

// IssueRefresh mints one refresh token in a new family for subject.
func (s *ReceiptStore) IssueRefresh(ctx context.Context, subject, tenant string) (familyID, token string, err error) {
	if s == nil || s.db == nil || s.clk == nil {
		return "", "", ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return "", "", fmt.Errorf("jwt: issue refresh: %w", err)
	}

	if subject == "" || tenant == "" {
		return "", "", fmt.Errorf("%w: subject and tenant are required", ErrInvalidClaims)
	}

	if _, err := valueobject.ParseTenantID(tenant); err != nil {
		return "", "", fmt.Errorf("%w: tenant: %s", ErrInvalidClaims, err.Error())
	}

	familyID = "fam_" + newOpaqueID()
	seed := newOpaqueID()
	token = "rt_" + seed + "." + tenant + "." + tokenChecksum(seed+tenant)
	now := s.now()

	err = postgres.WithinTenantTx(ctx, s.db, valueobject.TenantID(tenant), func(tx *gorm.DB) error {
		if err := tx.Create(&refreshFamilyModel{
			FamilyID: familyID,
			TenantID: tenant,
			Subject:  subject,
			IssuedAt: now,
		}).Error; err != nil {
			return err
		}

		return tx.Create(&refreshTokenModel{
			TokenHash: hashToken(token),
			FamilyID:  familyID,
			TenantID:  tenant,
		}).Error
	})

	if err != nil {
		return "", "", err
	}

	return familyID, token, nil
}

// spendFailure carries a domain failure out of the transaction callback:
// returning it directly would roll back the revocation writes it must commit.
type spendFailure struct{ err error }

func (e *spendFailure) Error() string { return e.err.Error() }

func (e *spendFailure) Unwrap() error { return e.err }

// UseRefresh spends one refresh token once. A replay revokes the family.
// The spend (lock family row → validate → mark used → mint successor) runs
// in one transaction, so concurrent spends serialize on the row lock and the
// token-hash primary key instead of on an app mutex.
func (s *ReceiptStore) UseRefresh(ctx context.Context, token string) (RefreshClaim, error) {
	if s == nil || s.db == nil || s.clk == nil {
		return RefreshClaim{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return RefreshClaim{}, fmt.Errorf("jwt: use refresh: %w", err)
	}

	tenantHint, err := parseRefreshToken(token)
	if err != nil {
		return RefreshClaim{}, err
	}

	tenant, err := valueobject.ParseTenantID(tenantHint)
	if err != nil {
		return RefreshClaim{}, ErrRefreshNotFound
	}

	var claim RefreshClaim

	var failure *spendFailure

	txErr := postgres.WithinTenantTx(ctx, s.db, tenant, func(tx *gorm.DB) error {
		c, err := s.spend(ctx, tx, tenant, token)
		if err != nil {
			if errors.As(err, &failure) {
				return nil
			}

			return err
		}

		claim = c

		return nil
	})

	if txErr != nil {
		return RefreshClaim{}, txErr
	}

	if failure != nil {
		return RefreshClaim{}, failure.err
	}

	return claim, nil
}

// spend executes one spend inside the caller's transaction: lock the receipt
// and family rows, validate, mark used, and mint the successor token.
func (s *ReceiptStore) spend(ctx context.Context, tx *gorm.DB, tenant valueobject.TenantID, token string) (RefreshClaim, error) {
	if err := ctx.Err(); err != nil {
		return RefreshClaim{}, err
	}

	receipt, family, err := s.lockLineage(tx, tenant, hashToken(token))
	if err != nil {
		return RefreshClaim{}, err
	}

	if family.Revoked {
		return RefreshClaim{}, &spendFailure{err: ErrRefreshRevoked}
	}

	if receipt.Used {
		if err := s.revokeFamily(tx, family.FamilyID); err != nil {
			return RefreshClaim{}, err
		}

		return RefreshClaim{}, &spendFailure{err: ErrRefreshReuse}
	}

	if s.now().After(family.IssuedAt.Add(s.ttl)) {
		if err := s.revokeFamily(tx, family.FamilyID); err != nil {
			return RefreshClaim{}, err
		}

		return RefreshClaim{}, &spendFailure{err: ErrExpired}
	}

	nextSeed := newOpaqueID()
	next := "rt_" + nextSeed + "." + string(tenant) + "." + tokenChecksum(nextSeed+string(tenant))

	if err := tx.Model(&refreshTokenModel{}).
		Where("token_hash = ?", receipt.TokenHash).
		Update("used", true).Error; err != nil {
		return RefreshClaim{}, err
	}

	if err := tx.Create(&refreshTokenModel{
		TokenHash: hashToken(next),
		FamilyID:  family.FamilyID,
		TenantID:  string(tenant),
	}).Error; err != nil {
		return RefreshClaim{}, err
	}

	return RefreshClaim{
		FamilyID:  family.FamilyID,
		Subject:   family.Subject,
		TenantID:  family.TenantID,
		IssuedAt:  s.now(),
		ExpiresAt: family.IssuedAt.Add(s.ttl),
		NextToken: next,
	}, nil
}

// lockLineage locks the receipt and family rows for one spend. A missing
// receipt is an unknown token; a missing family is a revoked lineage.
func (s *ReceiptStore) lockLineage(tx *gorm.DB, tenant valueobject.TenantID, digest string) (refreshTokenModel, refreshFamilyModel, error) {
	var receipt refreshTokenModel

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("token_hash = ? AND tenant_id = ?", digest, string(tenant)).
		First(&receipt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return receipt, refreshFamilyModel{}, &spendFailure{err: ErrRefreshNotFound}
		}

		return receipt, refreshFamilyModel{}, err
	}

	var family refreshFamilyModel

	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("family_id = ? AND tenant_id = ?", receipt.FamilyID, string(tenant)).
		First(&family).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return receipt, family, &spendFailure{err: ErrRefreshRevoked}
		}

		return receipt, family, err
	}

	return receipt, family, nil
}

// revokeFamily marks one lineage revoked inside the caller's transaction.
func (s *ReceiptStore) revokeFamily(tx *gorm.DB, familyID string) error {
	return tx.Model(&refreshFamilyModel{}).
		Where("family_id = ?", familyID).
		Update("revoked", true).Error
}

// hashToken hashes one opaque token with SHA256 (hex). Tokens are
// high-entropy random values; SHA256 binds them to rows without reversible
// storage. Keyed HMAC is unnecessary: there is no attacker-controlled input
// to separate from a secret here.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))

	return hex.EncodeToString(sum[:])
}

// parseRefreshToken extracts the tenant hint from rt_RAND.TENANT.CHECKSUM.
func parseRefreshToken(token string) (string, error) {
	rest, ok := strings.CutPrefix(token, "rt_")
	if !ok {
		return "", ErrRefreshNotFound
	}

	parts := strings.Split(rest, ".")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", ErrRefreshNotFound
	}

	if tokenChecksum(parts[0]+parts[1]) != parts[2] {
		return "", ErrRefreshNotFound
	}

	return parts[1], nil
}

// tokenChecksum is format integrity for refresh tokens (not secrecy).
func tokenChecksum(seed string) string {
	sum := sha256.Sum256([]byte(seed))

	return hex.EncodeToString(sum[:])[:8]
}

// newOpaqueID returns one random 128-bit hex token.
func newOpaqueID() string {
	var buf [16]byte

	_, _ = rand.Read(buf[:])

	return hex.EncodeToString(buf[:])
}
