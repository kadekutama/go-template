package audit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/rls"
)

// LokiExporter ships entries to Loki (hot trail).
type LokiExporter interface {
	ExportAudit(ctx context.Context, entry StoredEntry) error
}

// WORMExporter archives signed checkpoints to immutable cold storage.
type WORMExporter interface {
	ArchiveCheckpoint(ctx context.Context, root SignedRoot) error
}

// AuditParams carries constructor dependencies (Parameter Object pattern).
type AuditParams struct {
	DB         *gorm.DB
	SigningKey []byte `validate:"required,min=32"`
	Clock      appport.Clock
	Loki       LokiExporter
	WORM       WORMExporter
}

// auditRowModel maps audit_entries (migration 20260923000008).
type auditRowModel struct {
	TenantID   string    `gorm:"column:tenant_id;primaryKey"`
	Seq        int64     `gorm:"column:seq;primaryKey"`
	Actor      string    `gorm:"column:actor"`
	Action     string    `gorm:"column:action"`
	Resource   string    `gorm:"column:resource"`
	BeforeHash string    `gorm:"column:before_hash"`
	AfterHash  string    `gorm:"column:after_hash"`
	OccurredAt time.Time `gorm:"column:occurred_at"`
	PrevHash   string    `gorm:"column:prev_hash"`
	EntryHash  string    `gorm:"column:entry_hash"`
	Signature  string    `gorm:"column:signature"`
}

// TableName pins the model to the migrated table.
func (auditRowModel) TableName() string { return "audit_entries" }

// auditHeadModel maps audit_heads (migration 20260923000008).
type auditHeadModel struct {
	TenantID  string    `gorm:"column:tenant_id;primaryKey"`
	LastSeq   int64     `gorm:"column:last_seq"`
	LastHash  string    `gorm:"column:last_hash"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

// TableName pins the model to the migrated table.
func (auditHeadModel) TableName() string { return "audit_heads" }

// Logger appends tamper-evident audit facts per tenant into PostgreSQL.
// The table is the source of truth: restarts and replicas change nothing.
// Per-tenant appends serialize on row-level locks on audit_heads via SELECT FOR UPDATE,
// fully compatible with Citus sharding by tenant_id. The adapter itself is stateless.
type Logger struct {
	db         *gorm.DB
	signingKey []byte
	clock      appport.Clock
	loki       LokiExporter
	worm       WORMExporter
}

// Compile-time port conformance.
var _ appport.AuditLogger = (*Logger)(nil)

// NewLogger builds the audit adapter; DB, SigningKey, and Clock are required.
func NewLogger(params AuditParams) (*Logger, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("%w: DB is required", ErrConfigRequired)
	}

	if len(params.SigningKey) < 32 {
		return nil, ErrConfigRequired
	}

	if params.Clock == nil {
		return nil, ErrClockRequired
	}

	return &Logger{
		db:         params.DB,
		signingKey: append([]byte(nil), params.SigningKey...),
		clock:      params.Clock,
		loki:       params.Loki,
		worm:       params.WORM,
	}, nil
}

// now returns the current UTC time through the injected clock.
func (l *Logger) now() time.Time {
	return l.clock.Now().UTC()
}

// Record appends one audit fact with chain + signature. The Loki export runs
// after commit so a slow sink can never stall appends. Standalone calls open
// their own transaction; command handlers that must commit the fact with
// their writes use RecordTx inside the UnitOfWork callback instead.
func (l *Logger) Record(ctx context.Context, entry appport.AuditEntry) error {
	if l == nil || l.db == nil {
		return ErrNotInitialized
	}

	tenant, occurred, err := l.prepare(ctx, entry)
	if err != nil {
		return err
	}

	var stored StoredEntry

	err = postgres.WithinTenantTx(ctx, l.db, valueobject.TenantID(tenant), func(tx *gorm.DB) error {
		row, err := l.append(ctx, tx, tenant, entry, occurred)
		if err != nil {
			return err
		}

		stored = row

		return nil
	})
	if err != nil {
		return err
	}

	if l.loki != nil {
		if err := l.loki.ExportAudit(ctx, stored); err != nil {
			return fmt.Errorf("audit: loki export: %w", err)
		}
	}

	return nil
}

// RecordTx appends one audit fact inside the caller's transaction (the
// UnitOfWork callback's tx): the fact commits atomically with the command's
// writes or rolls back with them — never an independent commit. The Loki
// export still runs after commit via the caller's own post-commit hook;
// RecordTx performs the durable write only.
func (l *Logger) RecordTx(ctx context.Context, tx *gorm.DB, entry appport.AuditEntry) error {
	if tx == nil {
		return fmt.Errorf("%w: transaction is required", ErrConfigRequired)
	}

	tenant, occurred, err := l.prepare(ctx, entry)
	if err != nil {
		return err
	}

	_, err = l.append(ctx, tx, tenant, entry, occurred)

	return err
}

// prepare validates one entry without touching storage: nil guards, context,
// secret screen, tenant shape, and timestamp default. Both Record and
// RecordTx validate before any SQL so bad input never opens a transaction.
func (l *Logger) prepare(ctx context.Context, entry appport.AuditEntry) (string, time.Time, error) {
	if l == nil || l.db == nil || l.clock == nil || len(l.signingKey) == 0 {
		return "", time.Time{}, ErrNotInitialized
	}

	if err := ctx.Err(); err != nil {
		return "", time.Time{}, fmt.Errorf("audit: record: %w", err)
	}

	if err := fromPort(entry); err != nil {
		return "", time.Time{}, err
	}

	if _, err := valueobject.ParseTenantID(string(entry.TenantID)); err != nil {
		return "", time.Time{}, fmt.Errorf("%w: tenant: %s", ErrConfigRequired, err.Error())
	}

	occurred := entry.OccurredAt.UTC()
	if occurred.IsZero() {
		occurred = l.now()
	}

	// PostgreSQL timestamptz stores microseconds: normalize before hashing so
	// Verify recomputes over exactly the persisted precision (CR-002). Without
	// this, any sub-microsecond timestamp would false-alarm as tampering.
	occurred = occurred.Truncate(time.Microsecond)

	return string(entry.TenantID), occurred, nil
}

// append inserts one chained row under the per-tenant row lock on audit_heads,
// executing on the given handle: the standalone transaction for Record, or
// the caller's UnitOfWork tx for RecordTx.
func (l *Logger) append(ctx context.Context, db *gorm.DB, tenant string, entry appport.AuditEntry, occurred time.Time) (StoredEntry, error) {
	if err := rls.ApplyTenant(ctx, db, valueobject.TenantID(tenant)); err != nil {
		return StoredEntry{}, err
	}

	var head auditHeadModel
	lookup := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenant).First(&head)

	if lookup.Error != nil {
		if !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
			return StoredEntry{}, lookup.Error
		}

		// First append for this tenant: two concurrent first-appends would
		// both insert the head row. ON CONFLICT DO NOTHING serializes them on
		// the speculative-insertion lock (a concurrent inserter either commits
		// and we read its row, or aborts and our insert wins), then re-read
		// under lock — no application retry needed.
		head = auditHeadModel{
			TenantID:  tenant,
			LastSeq:   0,
			LastHash:  "genesis:" + tenant,
			UpdatedAt: occurred,
		}
		if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&head).Error; err != nil {
			return StoredEntry{}, err
		}

		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ?", tenant).First(&head).Error; err != nil {
			return StoredEntry{}, err
		}
	}

	prev := head.LastHash
	seq := head.LastSeq + 1

	entryHash := hashEntry(tenant, entry.Actor, entry.Action, entry.Resource, entry.BeforeHash, entry.AfterHash, occurred, prev, seq)

	row := auditRowModel{
		TenantID:   tenant,
		Seq:        seq,
		Actor:      entry.Actor,
		Action:     entry.Action,
		Resource:   entry.Resource,
		BeforeHash: entry.BeforeHash,
		AfterHash:  entry.AfterHash,
		OccurredAt: occurred,
		PrevHash:   prev,
		EntryHash:  entryHash,
		Signature:  signHash(l.signingKey, entryHash),
	}

	if err := db.Create(&row).Error; err != nil {
		return StoredEntry{}, err
	}

	if err := db.Model(&auditHeadModel{}).Where("tenant_id = ?", tenant).Updates(map[string]any{
		"last_seq":   seq,
		"last_hash":  entryHash,
		"updated_at": occurred,
	}).Error; err != nil {
		return StoredEntry{}, err
	}

	return toStored(row), nil
}

// Entries returns a copy of one tenant chain for verification/export.
func (l *Logger) Entries(ctx context.Context, tenant string) ([]StoredEntry, error) {
	if l == nil || l.db == nil {
		return nil, ErrNotInitialized
	}

	tenantID, err := valueobject.ParseTenantID(tenant)
	if err != nil {
		return nil, fmt.Errorf("%w: tenant: %s", ErrConfigRequired, err.Error())
	}

	var rows []auditRowModel

	if err := postgres.WithinTenantTx(ctx, l.db, tenantID, func(tx *gorm.DB) error {
		return tx.Where("tenant_id = ?", tenant).Order("seq ASC").Find(&rows).Error
	}); err != nil {
		return nil, err
	}

	out := make([]StoredEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, toStored(row))
	}

	return out, nil
}

// Verify verifies the cryptographic hash chain and signatures for tenant.
func (l *Logger) Verify(ctx context.Context, tenant string) error {
	if l == nil || l.db == nil || len(l.signingKey) == 0 {
		return ErrNotInitialized
	}

	entries, err := l.Entries(ctx, tenant)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		return nil
	}

	expectedPrev := "genesis:" + tenant
	for i, entry := range entries {
		expectedSeq := int64(i + 1)
		if entry.Seq != expectedSeq {
			return fmt.Errorf("%w: seq mismatch at index %d (got %d, want %d)", ErrChainBroken, i, entry.Seq, expectedSeq)
		}

		if entry.PrevHash != expectedPrev {
			return fmt.Errorf("%w: broken prev hash at seq %d", ErrChainBroken, entry.Seq)
		}

		recomputed := hashEntry(tenant, entry.Actor, entry.Action, entry.Resource, entry.BeforeHash, entry.AfterHash, entry.OccurredAt, entry.PrevHash, entry.Seq)
		if entry.EntryHash != recomputed {
			return fmt.Errorf("%w: hash mismatch at seq %d", ErrChainBroken, entry.Seq)
		}

		if !verifySignature(l.signingKey, entry.EntryHash, entry.Signature) {
			return fmt.Errorf("%w: invalid signature at seq %d", ErrChainBroken, entry.Seq)
		}

		expectedPrev = entry.EntryHash
	}

	return nil
}

// toStored maps one database row onto the exported chain view.
func toStored(row auditRowModel) StoredEntry {
	return StoredEntry{
		Seq:        row.Seq,
		Tenant:     row.TenantID,
		Actor:      row.Actor,
		Action:     row.Action,
		Resource:   row.Resource,
		BeforeHash: row.BeforeHash,
		AfterHash:  row.AfterHash,
		OccurredAt: row.OccurredAt,
		PrevHash:   row.PrevHash,
		EntryHash:  row.EntryHash,
		Signature:  row.Signature,
	}
}
