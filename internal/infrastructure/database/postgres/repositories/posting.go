package repositories

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
	"github.com/kadekutama/go-template/pkg/jsonparser"
)

// PostingRepositoryParams carries constructor dependencies.
type PostingRepositoryParams struct {
	DB *gorm.DB
}

// PostingRepository is the PostgreSQL PostingRepository + EntryReader adapter.
// Commit is the single atomic write path for postings + entries.
type PostingRepository struct {
	db *gorm.DB
}

// NewPostingRepository builds the adapter; DB must be non-nil.
func NewPostingRepository(params PostingRepositoryParams) (*PostingRepository, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: posting repository needs DB")
	}

	return &PostingRepository{db: params.DB}, nil
}

// Commit atomically stores one posting with all its entries: scope checks,
// deterministic account locks, per-account sequence assignment, structural
// validation, per-asset balance, then a single commit of posting + entries.
// Anything failing persists nothing. The adapter owns account_seq numbering
// (per account, after the current max); caller-supplied values are replaced.
func (r *PostingRepository) Commit(ctx context.Context, posting entity.PostingData) (entity.PostingData, error) {
	var stored entity.PostingData

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := scopeTenant(ctx, tx, posting.TenantID); err != nil {
			return err
		}

		if err := lockPostingAccounts(ctx, tx, posting); err != nil {
			return err
		}

		sequenced, err := assignSequences(ctx, tx, posting)
		if err != nil {
			return err
		}

		if err := sequenced.Validate(); err != nil {
			return err
		}

		if err := checkBalanced(sequenced.Entries); err != nil {
			return err
		}

		postingModel := postingToModel(sequenced)

		if err := tx.Create(&postingModel).Error; err != nil {
			if isDuplicate(err) {
				return ErrConflict
			}

			return fmt.Errorf("postgres: insert posting: %w", err)
		}

		sequenced.ID = valueobject.PostingID(postingModel.ID)

		storedEntries, err := insertPostingEntries(ctx, tx, sequenced)
		if err != nil {
			return err
		}

		stored = postingToEntity(postingModel, storedEntries)

		return nil
	})
	if err != nil {
		return entity.PostingData{}, err
	}

	return stored, nil
}

// insertPostingEntries persists every entry with the stored posting identity
// and returns them with database-assigned IDs.
func insertPostingEntries(ctx context.Context, tx *gorm.DB, sequenced entity.PostingData) ([]entity.Entry, error) {
	entryModels := make([]models.EntryModel, 0, len(sequenced.Entries))

	for _, entry := range sequenced.Entries {
		entryModel := entryToModel(sequenced, entry)
		if err := tx.WithContext(ctx).Create(&entryModel).Error; err != nil {
			if isDuplicate(err) {
				return nil, ErrConflict
			}

			return nil, fmt.Errorf("postgres: insert entry: %w", err)
		}

		entryModels = append(entryModels, entryModel)
	}

	stored := make([]entity.Entry, 0, len(entryModels))
	for _, entryModel := range entryModels {
		stored = append(stored, entryToEntity(entryModel))
	}

	return stored, nil
}

// CursorOf returns the ledger cursor (ledger_seq) of a committed posting.
func (r *PostingRepository) CursorOf(ctx context.Context, id valueobject.PostingID) (string, error) {
	var seq int64

	err := r.db.WithContext(ctx).Model(&models.PostingModel{}).
		Where("id = ?", id.String()).Pluck("ledger_seq", &seq).Error
	if err != nil {
		return "", fmt.Errorf("postgres: posting cursor: %w", err)
	}

	return strconv.FormatInt(seq, 10), nil
}

// accountLock is the scope/status projection locked per posting account.
type accountLock struct {
	ID       string
	TenantID string `gorm:"column:tenant_id"`
	LedgerID string `gorm:"column:ledger_id"`
	Status   string
}

// lockPostingAccounts locks every involved account in deterministic (sorted)
// order and verifies same tenant/ledger scope plus ACTIVE status.
func lockPostingAccounts(ctx context.Context, tx *gorm.DB, posting entity.PostingData) error {
	ids := make([]string, 0, len(posting.Entries))
	seen := make(map[string]bool)

	for _, entry := range posting.Entries {
		id := entry.AccountID.String()
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}

	sort.Strings(ids)

	var rows []accountLock

	// One ordered SELECT ... FOR UPDATE: deterministic acquisition order
	// prevents deadlocks between concurrent postings sharing accounts.
	if err := tx.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Model(&models.AccountModel{}).
		Where("id IN ?", ids).Order("id").
		Find(&rows).Error; err != nil {
		return fmt.Errorf("postgres: lock accounts: %w", err)
	}

	byID := make(map[string]accountLock, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}

	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			return fmt.Errorf("postgres: unknown account %s", id)
		}

		if row.TenantID != posting.TenantID.String() || row.LedgerID != posting.LedgerID.String() {
			return fmt.Errorf("postgres: account %s outside posting scope", id)
		}

		if row.Status != string(valueobject.StatusActive) {
			return fmt.Errorf("postgres: account %s is not ACTIVE", id)
		}
	}

	return nil
}

// assignSequences numbers entries per account after the current max so every
// row carries a monotonic account_seq without caller coordination.
func assignSequences(ctx context.Context, tx *gorm.DB, posting entity.PostingData) (entity.PostingData, error) {
	next := make(map[string]int64)

	for index := range posting.Entries {
		account := posting.Entries[index].AccountID.String()

		base, ok := next[account]
		if !ok {
			var max int64

			err := tx.WithContext(ctx).Model(&models.EntryModel{}).
				Where("tenant_id = ? AND account_id = ?", posting.TenantID.String(), account).
				Select("COALESCE(MAX(account_seq), 0)").Scan(&max).Error
			if err != nil {
				return entity.PostingData{}, fmt.Errorf("postgres: sequence base: %w", err)
			}

			base = max
		}

		base++
		next[account] = base
		posting.Entries[index].AccountSeq = base
	}

	return posting, nil
}

// FindByID returns one posting with entries by tenant + ID (strong read).
func (r *PostingRepository) FindByID(
	ctx context.Context,
	tenant valueobject.TenantID,
	id valueobject.PostingID,
) (entity.PostingData, error) {
	if tenant.String() == "" {
		return entity.PostingData{}, fmt.Errorf("postgres: tenant is required")
	}

	var posting models.PostingModel

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return entity.PostingData{}, err
	}

	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id.String(), tenant.String()).First(&posting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.PostingData{}, ErrNotFound
		}

		return entity.PostingData{}, fmt.Errorf("postgres: find posting: %w", err)
	}

	entries, err := findEntriesByPosting(ctx, r.db, tenant, id)
	if err != nil {
		return entity.PostingData{}, err
	}

	return postingToEntity(posting, entries), nil
}

// FindByExternalReference returns the posting filed under a caller reference.
func (r *PostingRepository) FindByExternalReference(
	ctx context.Context,
	tenant valueobject.TenantID,
	reference string,
) (entity.PostingData, error) {
	if tenant.String() == "" {
		return entity.PostingData{}, fmt.Errorf("postgres: tenant is required")
	}

	var posting models.PostingModel

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return entity.PostingData{}, err
	}

	err := r.db.WithContext(ctx).Where("tenant_id = ? AND external_reference = ?", tenant.String(), reference).First(&posting).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return entity.PostingData{}, ErrNotFound
		}

		return entity.PostingData{}, fmt.Errorf("postgres: find posting by reference: %w", err)
	}

	entries, err := findEntriesByPosting(ctx, r.db, tenant, valueobject.PostingID(posting.ID))
	if err != nil {
		return entity.PostingData{}, err
	}

	return postingToEntity(posting, entries), nil
}

// FindByAccount returns one opaque-cursor page of postings touching an
// account. The cursor is the last seen entry account_seq (decimal string);
// pages follow entry insertion order. Postings are assembled in batch
// (one postings query + one entries query, no N+1).
func (r *PostingRepository) FindByAccount(
	ctx context.Context,
	tenant valueobject.TenantID,
	account valueobject.AccountID,
	cursor string,
	limit int,
) ([]entity.PostingData, string, error) {
	if tenant.String() == "" {
		return nil, "", fmt.Errorf("postgres: tenant is required")
	}

	if limit <= 0 || limit > 100 {
		limit = 50
	}

	after, err := parseSeqCursor(cursor)
	if err != nil {
		return nil, "", err
	}

	if err := scopeTenant(ctx, r.db, tenant); err != nil {
		return nil, "", err
	}

	var refs []entryRef

	if err := r.db.WithContext(ctx).Model(&models.EntryModel{}).
		Where("tenant_id = ? AND account_id = ? AND account_seq > ?", tenant.String(), account.String(), after).
		Order("account_seq").Limit(limit+1).
		Select("posting_id", "account_seq").Scan(&refs).Error; err != nil {
		return nil, "", fmt.Errorf("postgres: list postings by account: %w", err)
	}

	next := ""

	if len(refs) > limit {
		next = strconv.FormatInt(refs[limit-1].AccountSeq, 10)
		refs = refs[:limit]
	}

	ids := make([]string, 0, len(refs))
	seen := make(map[string]bool)

	for _, ref := range refs {
		if !seen[ref.PostingID] {
			seen[ref.PostingID] = true
			ids = append(ids, ref.PostingID)
		}
	}

	if len(ids) == 0 {
		return []entity.PostingData{}, "", nil
	}

	return assemblePage(ctx, r.db, tenant, ids, next)
}

// assemblePage batch-fetches postings + entries (no N+1) in id order.
func assemblePage(
	ctx context.Context,
	db *gorm.DB,
	tenant valueobject.TenantID,
	ids []string,
	next string,
) ([]entity.PostingData, string, error) {
	var postingRows []models.PostingModel

	if err := db.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenant.String(), ids).Find(&postingRows).Error; err != nil {
		return nil, "", fmt.Errorf("postgres: fetch postings page: %w", err)
	}

	var entryRows []models.EntryModel

	if err := db.WithContext(ctx).Where("tenant_id = ? AND posting_id IN ?", tenant.String(), ids).
		Order("account_seq").Find(&entryRows).Error; err != nil {
		return nil, "", fmt.Errorf("postgres: fetch page entries: %w", err)
	}

	byPosting := make(map[string][]entity.Entry, len(ids))
	for _, row := range entryRows {
		byPosting[row.PostingID] = append(byPosting[row.PostingID], entryToEntity(row))
	}

	byID := make(map[string]models.PostingModel, len(postingRows))
	for _, row := range postingRows {
		byID[row.ID] = row
	}

	out := make([]entity.PostingData, 0, len(ids))

	for _, id := range ids {
		row, ok := byID[id]
		if !ok {
			return nil, "", fmt.Errorf("postgres: posting %s vanished mid-page", id)
		}

		out = append(out, postingToEntity(row, byPosting[id]))
	}

	return out, next, nil
}

// entryRef is one ordered posting touch for pagination.
type entryRef struct {
	PostingID  string
	AccountSeq int64
}

// parseSeqCursor decodes the opaque account_seq cursor ("" starts at zero).
func parseSeqCursor(cursor string) (int64, error) {
	if cursor == "" {
		return 0, nil
	}

	seq, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil || seq < 0 {
		return 0, fmt.Errorf("postgres: invalid cursor")
	}

	return seq, nil
}

// findEntriesByPosting lists entries of one posting on any handle (strong read).
func findEntriesByPosting(
	ctx context.Context,
	db *gorm.DB,
	tenant valueobject.TenantID,
	posting valueobject.PostingID,
) ([]entity.Entry, error) {
	if tenant.String() == "" {
		return nil, fmt.Errorf("postgres: tenant is required")
	}

	if err := scopeTenant(ctx, db, tenant); err != nil {
		return nil, err
	}

	var rows []models.EntryModel

	if err := db.WithContext(ctx).
		Where("tenant_id = ? AND posting_id = ?", tenant.String(), posting.String()).
		Order("account_seq").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("postgres: list entries: %w", err)
	}

	out := make([]entity.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, entryToEntity(row))
	}

	return out, nil
}

// checkBalanced enforces per-asset double-entry before the write.
func checkBalanced(entries []entity.Entry) error {
	nets := make(map[valueobject.AssetCode]int64)

	for _, entry := range entries {
		delta := entry.AmountMinor
		if entry.Side == valueobject.DirectionCredit {
			delta = -delta
		}

		sum, overflow := addChecked(nets[entry.AssetCode], delta)
		if overflow {
			return fmt.Errorf("postgres: balance overflow for asset %s", entry.AssetCode)
		}

		nets[entry.AssetCode] = sum
	}

	for asset, net := range nets {
		if net != 0 {
			return fmt.Errorf("postgres: unbalanced posting for asset %s", asset)
		}
	}

	return nil
}

// addChecked adds with overflow detection.
func addChecked(left int64, right int64) (int64, bool) {
	sum := left + right
	if (right > 0 && sum < left) || (right < 0 && sum > left) {
		return 0, true
	}

	return sum, false
}

func postingToModel(posting entity.PostingData) models.PostingModel {
	var reversal *string
	if posting.ReversalOf != nil {
		reversed := posting.ReversalOf.String()
		reversal = &reversed
	}

	return models.PostingModel{
		ID:                posting.ID.String(),
		TenantID:          posting.TenantID.String(),
		LedgerID:          posting.LedgerID.String(),
		Operation:         posting.Operation,
		ExternalReference: posting.ExternalReference,
		Description:       posting.Description,
		EffectiveAt:       posting.EffectiveAt,
		RecordedAt:        posting.RecordedAt,
		ReversalOf:        reversal,
		Reason:            posting.Reason,
		Metadata:          marshalMetadata(posting.Metadata),
	}
}

// marshalMetadata encodes domain metadata as a JSONB document ('{}' when
// empty) through the repository codec seam (pkg/jsonparser, never encoding/json directly).
func marshalMetadata(metadata map[string]string) string {
	if len(metadata) == 0 {
		return "{}"
	}

	raw, err := jsonparser.Marshal(metadata)
	if err != nil {
		return "{}"
	}

	return string(raw)
}

func entryToModel(posting entity.PostingData, entry entity.Entry) models.EntryModel {
	return models.EntryModel{
		ID:          entry.ID.String(),
		PostingID:   posting.ID.String(),
		TenantID:    posting.TenantID.String(),
		LedgerID:    posting.LedgerID.String(),
		AccountID:   entry.AccountID.String(),
		Side:        string(entry.Side),
		AmountMinor: entry.AmountMinor,
		AssetCode:   string(entry.AssetCode),
		AccountSeq:  entry.AccountSeq,
	}
}

func postingToEntity(model models.PostingModel, entries []entity.Entry) entity.PostingData {
	posting := entity.PostingData{
		ID:                valueobject.PostingID(model.ID),
		TenantID:          valueobject.TenantID(model.TenantID),
		LedgerID:          valueobject.LedgerID(model.LedgerID),
		Operation:         model.Operation,
		ExternalReference: model.ExternalReference,
		Description:       model.Description,
		Entries:           entries,
		EffectiveAt:       model.EffectiveAt,
		RecordedAt:        model.RecordedAt,
		Reason:            model.Reason,
	}

	if model.ReversalOf != nil && *model.ReversalOf != "" {
		reversal := valueobject.PostingID(*model.ReversalOf)
		posting.ReversalOf = &reversal
	}

	return posting
}

func entryToEntity(model models.EntryModel) entity.Entry {
	return entity.Entry{
		ID:          valueobject.EntryID(model.ID),
		PostingID:   valueobject.PostingID(model.PostingID),
		AccountID:   valueobject.AccountID(model.AccountID),
		Side:        valueobject.Direction(model.Side),
		AmountMinor: model.AmountMinor,
		AssetCode:   valueobject.AssetCode(model.AssetCode),
		AccountSeq:  model.AccountSeq,
	}
}

// Compile-time port assertion.
var _ repository.PostingRepository = (*PostingRepository)(nil)
