package repositories

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// WorkflowRecord is the persistence shape for operational workflows.
type WorkflowRecord struct {
	ID          string
	TenantID    string
	LedgerID    string
	Kind        string
	Status      string
	PostingID   string
	PayloadHash string
	Version     int64
}

// WorkflowStoreParams carries constructor dependencies.
type WorkflowStoreParams struct {
	DB *gorm.DB
}

// WorkflowStore persists operational workflows (migration 000003 tables).
// It writes owned tables only and can never mutate postings/entries.
type WorkflowStore struct {
	db *gorm.DB
}

// NewWorkflowStore builds the store; DB must be non-nil.
func NewWorkflowStore(params WorkflowStoreParams) (*WorkflowStore, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: workflow store needs DB")
	}

	return &WorkflowStore{db: params.DB}, nil
}

// Create stores a new workflow and returns it with the database-assigned
// ID; duplicate IDs conflict, cross-tenant references are rejected by the
// caller contract (tenant always required).
func (s *WorkflowStore) Create(ctx context.Context, record WorkflowRecord) (WorkflowRecord, error) {
	if record.TenantID == "" {
		return WorkflowRecord{}, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, s.db, valueobject.TenantID(record.TenantID)); err != nil {
		return WorkflowRecord{}, err
	}

	model := workflowToModel(record)

	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return WorkflowRecord{}, ErrConflict
		}

		return WorkflowRecord{}, fmt.Errorf("postgres: create workflow: %w", err)
	}

	return workflowToRecord(model), nil
}

// FindByID returns one workflow by tenant + ID (strong read).
func (s *WorkflowStore) FindByID(ctx context.Context, tenant string, id string) (WorkflowRecord, error) {
	if tenant == "" {
		return WorkflowRecord{}, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, s.db, valueobject.TenantID(tenant)); err != nil {
		return WorkflowRecord{}, err
	}

	var model models.WorkflowModel

	err := s.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenant).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return WorkflowRecord{}, ErrNotFound
		}

		return WorkflowRecord{}, fmt.Errorf("postgres: find workflow: %w", err)
	}

	return workflowToRecord(model), nil
}

// UpdateStatus transitions status guarded by the expected version.
func (s *WorkflowStore) UpdateStatus(ctx context.Context, tenant string, id string, status string, expectedVersion int64) error {
	if tenant == "" {
		return fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, s.db, valueobject.TenantID(tenant)); err != nil {
		return err
	}

	res := s.db.WithContext(ctx).Model(&models.WorkflowModel{}).
		Where("id = ? AND tenant_id = ? AND version = ?", id, tenant, expectedVersion).
		Updates(map[string]any{
			"status":        status,
			columnVersion:   expectedVersion + 1,
			columnUpdatedAt: gorm.Expr("now()"),
		})
	if res.Error != nil {
		return fmt.Errorf("postgres: update workflow: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return ErrVersionMismatch
	}

	return nil
}

// ReconSourceRecord retains an immutable provider source.
type ReconSourceRecord struct {
	ID            string
	TenantID      string
	Format        string
	PayloadHash   string
	ParserVersion string
	RawRef        string
}

// StoreReconSource persists a source and returns it with the
// database-assigned ID; identical tenant+hash dedupes.
func (s *WorkflowStore) StoreReconSource(ctx context.Context, record ReconSourceRecord) (ReconSourceRecord, error) {
	if record.TenantID == "" {
		return ReconSourceRecord{}, fmt.Errorf("postgres: tenant is required")
	}

	if record.PayloadHash == "" {
		return ReconSourceRecord{}, fmt.Errorf("postgres: payload hash is required")
	}

	if err := scopeTenant(ctx, s.db, valueobject.TenantID(record.TenantID)); err != nil {
		return ReconSourceRecord{}, err
	}

	model := models.ReconSourceModel{
		ID:            record.ID,
		TenantID:      record.TenantID,
		Format:        record.Format,
		PayloadHash:   record.PayloadHash,
		ParserVersion: record.ParserVersion,
		RawRef:        record.RawRef,
	}

	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return ReconSourceRecord{}, ErrConflict
		}

		return ReconSourceRecord{}, fmt.Errorf("postgres: store recon source: %w", err)
	}

	record.ID = model.ID

	return record, nil
}

// InboxReceipt records a consumed event key; replays dedupe on (tenant, key).
func (s *WorkflowStore) InboxReceipt(ctx context.Context, tenant string, key string, eventType string) error {
	if tenant == "" {
		return fmt.Errorf("postgres: tenant is required")
	}

	if key == "" {
		return fmt.Errorf("postgres: inbox key is required")
	}

	if err := scopeTenant(ctx, s.db, valueobject.TenantID(tenant)); err != nil {
		return err
	}

	model := models.InboxReceiptModel{TenantID: tenant, Key: key, EventType: eventType}

	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicate(err) {
			return ErrConflict
		}

		return fmt.Errorf("postgres: inbox receipt: %w", err)
	}

	return nil
}

func workflowToModel(record WorkflowRecord) models.WorkflowModel {
	model := models.WorkflowModel{
		ID:          record.ID,
		TenantID:    record.TenantID,
		Kind:        record.Kind,
		Status:      record.Status,
		PayloadHash: record.PayloadHash,
		Version:     1,
	}

	if record.LedgerID != "" {
		ledger := record.LedgerID
		model.LedgerID = &ledger
	}

	if record.PostingID != "" {
		posting := record.PostingID
		model.PostingID = &posting
	}

	if record.Version >= 1 {
		model.Version = record.Version
	}

	return model
}

func workflowToRecord(model models.WorkflowModel) WorkflowRecord {
	record := WorkflowRecord{
		ID:          model.ID,
		TenantID:    model.TenantID,
		Kind:        model.Kind,
		Status:      model.Status,
		PayloadHash: model.PayloadHash,
		Version:     model.Version,
	}

	if model.LedgerID != nil {
		record.LedgerID = *model.LedgerID
	}

	if model.PostingID != nil {
		record.PostingID = *model.PostingID
	}

	return record
}
