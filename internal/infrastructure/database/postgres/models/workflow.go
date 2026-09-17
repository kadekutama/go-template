package models

import (
	"time"

	"gorm.io/gorm"
)

// TenantModel is the GORM row for tenancy records (migration 000003).
type TenantModel struct {
	ID        string `gorm:"primaryKey"`
	Name      string
	Region    string
	Settings  string `gorm:"column:settings;type:jsonb"`
	Status    string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table for migration 000003.
func (TenantModel) TableName() string { return "tenants" }

// BeforeCreate ensures settings is a valid JSON document ('{}' when empty),
// preventing PostgreSQL 22P02 invalid input syntax for type json.
func (m *TenantModel) BeforeCreate(_ *gorm.DB) error {
	if m.Settings == "" {
		m.Settings = "{}"
	}
	return nil
}

// BeforeUpdate ensures settings is a valid JSON document ('{}' when empty).
func (m *TenantModel) BeforeUpdate(_ *gorm.DB) error {
	if m.Settings == "" {
		m.Settings = "{}"
	}
	return nil
}

// WorkflowModel is the GORM row for operational workflows (migration 000003).
// It references postings but never writes them.
type WorkflowModel struct {
	ID          string `gorm:"primaryKey"`
	TenantID    string `gorm:"column:tenant_id;index"`
	LedgerID    *string
	Kind        string
	Status      string
	PostingID   *string
	PayloadHash string `gorm:"column:payload_hash"`
	Version     int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName pins the table for migration 000003.
func (WorkflowModel) TableName() string { return "workflows" }

// ReconSourceModel retains immutable provider/statement sources (000003).
type ReconSourceModel struct {
	ID            string `gorm:"primaryKey"`
	TenantID      string `gorm:"column:tenant_id;index"`
	Format        string
	PayloadHash   string `gorm:"column:payload_hash"`
	ParserVersion string `gorm:"column:parser_version"`
	RawRef        string `gorm:"column:raw_ref"`
	ReceivedAt    time.Time
}

// TableName pins the table for migration 000003.
func (ReconSourceModel) TableName() string { return "recon_sources" }

// ReconMatchGroupModel groups matched lines for a period (migration 000003).
type ReconMatchGroupModel struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"column:tenant_id"`
	Period    string
	Status    string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table for migration 000003.
func (ReconMatchGroupModel) TableName() string { return "recon_match_groups" }

// ReconBreakModel is one unmatched/exception line (migration 000003).
type ReconBreakModel struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"column:tenant_id"`
	GroupID   string `gorm:"column:group_id;index"`
	BreakType string `gorm:"column:break_type"`
	Status    string
	Resolver  string
	Approver  string
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table for migration 000003.
func (ReconBreakModel) TableName() string { return "recon_breaks" }

// PeriodModel tracks open/close lifecycle (migration 000003).
type PeriodModel struct {
	ID       string `gorm:"primaryKey"`
	TenantID string `gorm:"column:tenant_id;index"`
	LedgerID *string
	Status   string
	Version  int64
	OpenedAt time.Time
	ClosedAt *time.Time
}

// TableName pins the table for migration 000003.
func (PeriodModel) TableName() string { return "periods" }

// ReportModel tracks generated reports (migration 000003).
type ReportModel struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"column:tenant_id"`
	Template  string
	Status    string
	SignedURL string `gorm:"column:signed_url"`
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName pins the table for migration 000003.
func (ReportModel) TableName() string { return "reports" }

// ApprovalModel records SoD approvals (migration 000003).
type ApprovalModel struct {
	ID        string `gorm:"primaryKey"`
	TenantID  string `gorm:"column:tenant_id"`
	SubjectID string `gorm:"column:subject_id"`
	Approver  string
	Decision  string
	CreatedAt time.Time
}

// TableName pins the table for migration 000003.
func (ApprovalModel) TableName() string { return "approvals" }

// InboxReceiptModel dedupes consumed events (migration 000003).
type InboxReceiptModel struct {
	TenantID   string `gorm:"column:tenant_id;primaryKey"`
	Key        string `gorm:"column:key;primaryKey"`
	EventType  string `gorm:"column:event_type"`
	ReceivedAt time.Time
}

// TableName pins the table for migration 000003.
func (InboxReceiptModel) TableName() string { return "inbox_receipts" }

// AuditRefModel records data-movement/audit references (migration 000003).
type AuditRefModel struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	TenantID  string
	Actor     string
	Action    string
	SubjectID string `gorm:"column:subject_id"`
	Residency string
	CreatedAt time.Time
}

// TableName pins the table for migration 000003.
func (AuditRefModel) TableName() string { return "audit_refs" }
