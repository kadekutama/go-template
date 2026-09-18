package models

import (
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// LedgerModel is the GORM row for entity.Ledger. Identity is a
// DB-generated native UUID (ADR-019); the composite tenant-first primary key
// mirrors migration 000001 for Citus co-location.
type LedgerModel struct {
	TenantID     string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID           string `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	Name         string
	Alias        string
	BaseAsset    string `gorm:"column:base_asset"`
	ChartVersion string `gorm:"column:chart_version"`
	CreatedAt    time.Time
}

// TableName pins the table for migration 000001.
func (LedgerModel) TableName() string { return "ledgers" }

// AssetModel is the GORM row for currency/asset definition (migration 000001).
type AssetModel struct {
	Code      string `gorm:"primaryKey"`
	Precision int    `gorm:"column:precision"`
	Status    string `gorm:"column:status"`
	CreatedAt time.Time
}

// TableName pins the table for migration 000001.
func (AssetModel) TableName() string { return "assets" }

// LedgerToModel maps domain to row. An empty ID is omitted so the database
// assigns a UUIDv7 identity (ADR-019); explicit IDs pass through for
// fixtures and idempotent replays.
func LedgerToModel(ledger entity.Ledger) LedgerModel {
	return LedgerModel{
		ID:           ledger.ID.String(),
		TenantID:     ledger.TenantID.String(),
		Name:         ledger.Name,
		Alias:        ledger.Alias,
		BaseAsset:    string(ledger.BaseAsset),
		ChartVersion: ledger.ChartVersion,
	}
}

// LedgerToEntity maps a row to domain, surfacing invalid stored rows.
func LedgerToEntity(model LedgerModel) (entity.Ledger, error) {
	if strings.TrimSpace(model.ID) == "" {
		return entity.Ledger{}, entity.NewError("LEDGER_ID_REQUIRED", "ledger id is required")
	}
	ledger, err := entity.NewLedger(
		valueobject.LedgerID(model.ID),
		valueobject.TenantID(model.TenantID),
		model.Name,
		valueobject.AssetCode(model.BaseAsset),
		model.ChartVersion,
	)
	if err != nil {
		return entity.Ledger{}, err
	}

	ledger.Alias = model.Alias

	return ledger, nil
}

// AccountModel is the GORM row for entity.AccountData. Balances are never
// stored here: they derive from entries minus holds.
type AccountModel struct {
	TenantID  string  `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID        string  `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	LedgerID  string  `gorm:"column:ledger_id;type:uuid;index"`
	ParentID  *string `gorm:"column:parent_id;type:uuid"`
	Number    string
	Name      string
	Class     string
	AssetCode string `gorm:"column:asset_code"`
	Status    string
	Purpose   string
	Metadata  string `gorm:"column:metadata;type:jsonb"`
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table for migration 000001.
func (AccountModel) TableName() string { return "accounts" }

// BeforeCreate ensures metadata is a valid JSON document ('{}' when empty),
// preventing PostgreSQL 22P02 invalid input syntax for type json.
func (m *AccountModel) BeforeCreate(_ *gorm.DB) error {
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	return nil
}

// BeforeUpdate ensures metadata is a valid JSON document ('{}' when empty).
func (m *AccountModel) BeforeUpdate(_ *gorm.DB) error {
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	return nil
}

// PostingModel is the GORM row for entity.PostingData (immutable fact).
type PostingModel struct {
	TenantID          string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID                string `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	LedgerID          string `gorm:"column:ledger_id;type:uuid;index"`
	Operation         string
	ExternalReference string `gorm:"column:external_reference;index"`
	Description       string
	EffectiveAt       time.Time
	RecordedAt        time.Time
	ReversalOf        *string `gorm:"column:reversal_of;type:uuid"`
	Reason            string
	Metadata          string `gorm:"column:metadata;type:jsonb"`
	LedgerSeq         int64  `gorm:"column:ledger_seq;autoIncrement"`
}

// TableName pins the table for migration 000001.
func (PostingModel) TableName() string { return "postings" }

// BeforeCreate ensures metadata is a valid JSON document ('{}' when empty),
// preventing PostgreSQL 22P02 invalid input syntax for type json.
func (m *PostingModel) BeforeCreate(_ *gorm.DB) error {
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	return nil
}

// BeforeUpdate ensures metadata is a valid JSON document ('{}' when empty).
func (m *PostingModel) BeforeUpdate(_ *gorm.DB) error {
	if m.Metadata == "" {
		m.Metadata = "{}"
	}
	return nil
}

// EntryModel is the GORM row for entity.Entry: BIGINT minor units only.
type EntryModel struct {
	TenantID    string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID          string `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	PostingID   string `gorm:"column:posting_id;type:uuid;index"`
	LedgerID    string `gorm:"column:ledger_id;type:uuid"`
	AccountID   string `gorm:"column:account_id;type:uuid;index"`
	Side        string
	AmountMinor int64  `gorm:"column:amount_minor"`
	AssetCode   string `gorm:"column:asset_code"`
	AccountSeq  int64  `gorm:"column:account_seq"`
}

// TableName pins the table for migration 000001.
func (EntryModel) TableName() string { return "entries" }

// HoldModel is the GORM row for entity.HoldData.
type HoldModel struct {
	TenantID    string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID          string `gorm:"type:uuid;primaryKey;default:uuidv7()"`
	LedgerID    string `gorm:"column:ledger_id;type:uuid"`
	AccountID   string `gorm:"column:account_id;type:uuid;index"`
	AssetCode   string `gorm:"column:asset_code"`
	AmountMinor int64  `gorm:"column:amount_minor"`
	Kind        string
	State       string
	ExpiresAt   time.Time
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName pins the table for migration 000001.
func (HoldModel) TableName() string { return "holds" }

// CheckpointModel is the rebuildable per-account/asset reference at a ledger
// cursor (ADR-011 interim posture). Derived, never spend authority.
type CheckpointModel struct {
	TenantID     string `gorm:"column:tenant_id;type:uuid;index;primaryKey"`
	LedgerID     string `gorm:"column:ledger_id;type:uuid;primaryKey"`
	AccountID    string `gorm:"column:account_id;type:uuid;primaryKey"`
	AssetCode    string `gorm:"column:asset_code;primaryKey"`
	CursorSeq    int64  `gorm:"column:cursor_seq;primaryKey"`
	BalanceMinor string `gorm:"column:balance_minor;type:numeric(38,0)"`
	CreatedAt    time.Time
}

// TableName pins the table for migration 000001.
func (CheckpointModel) TableName() string { return "checkpoints" }

// IdempotencyModel is the durable request-hash/result row (migration 000002).
type IdempotencyModel struct {
	TenantID    string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	Key         string `gorm:"column:key;primaryKey"`
	Fingerprint string
	Response    []byte
	CompletedAt *time.Time
	CreatedAt   time.Time
}

// TableName pins the table for migration 000002.
func (IdempotencyModel) TableName() string { return "idempotency_records" }

// OutboxModel is the transactional event fact row (migration 000002).
type OutboxModel struct {
	TenantID         string `gorm:"column:tenant_id;type:uuid;primaryKey"`
	ID               int64  `gorm:"column:id;primaryKey;autoIncrement"`
	LedgerID         string `gorm:"column:ledger_id;type:uuid"`
	EventType        string `gorm:"column:event_type"`
	AggregateID      string `gorm:"column:aggregate_id"`
	AggregateVersion int64  `gorm:"column:aggregate_version"`
	Payload          []byte
	OccurredAt       time.Time
	ClaimedAt        *time.Time
	DeliveredAt      *time.Time
}

// TableName pins the table for migration 000002.
func (OutboxModel) TableName() string { return "outbox_events" }
