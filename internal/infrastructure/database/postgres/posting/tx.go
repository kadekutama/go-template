// Package posting implements the explicit atomic posting transaction
// (E07-T01): idempotency reserve, deterministic account locks, scope and
// per-asset balance enforcement, then posting/entries/checkpoint/outbox
// commit exactly once behind port.UnitOfWork.
package posting

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/repository"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
	repos "github.com/kadekutama/go-template/internal/infrastructure/database/postgres/repositories"
)

// UnitOfWorkParams carries constructor dependencies.
type UnitOfWorkParams struct {
	DB *gorm.DB
}

// UnitOfWork is the PostgreSQL port.UnitOfWork adapter.
type UnitOfWork struct {
	db *gorm.DB
}

// NewUnitOfWork builds the adapter; DB must be non-nil.
func NewUnitOfWork(params UnitOfWorkParams) (*UnitOfWork, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("postgres: unit of work needs DB")
	}

	return &UnitOfWork{db: params.DB}, nil
}

// Do runs fn inside one atomic transaction; nil commits, non-nil rolls back.
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, tx appport.Tx) error) error {
	if fn == nil {
		return fmt.Errorf("postgres: unit of work callback is required")
	}

	return u.db.WithContext(ctx).Transaction(func(gormTx *gorm.DB) error {
		tx, err := newTx(gormTx)
		if err != nil {
			return err
		}

		return fn(ctx, tx)
	})
}

// tx is the atomic scope handed to one callback.
type tx struct {
	db          *gorm.DB
	postings    *txPostings
	holds       *repos.HoldRepository
	idempotency *idempotencyStore
	outbox      *outboxStore
	cursor      string
}

func newTx(db *gorm.DB) (*tx, error) {
	postings, err := repos.NewPostingRepository(repos.PostingRepositoryParams{DB: db})
	if err != nil {
		return nil, err
	}

	holds, err := repos.NewHoldRepository(repos.HoldRepositoryParams{DB: db})
	if err != nil {
		return nil, err
	}

	unit := &tx{
		db:          db,
		holds:       holds,
		idempotency: &idempotencyStore{db: db},
		outbox:      &outboxStore{db: db},
	}
	unit.postings = &txPostings{PostingRepository: postings, tx: db, onCursor: unit.setCursor}

	return unit, nil
}

// txPostings records the ledger cursor of every committed posting so strong
// reads can address this transaction exactly.
type txPostings struct {
	*repos.PostingRepository
	tx       *gorm.DB
	onCursor func(cursor string)
}

// Commit stores the posting, then captures its cursor in-transaction.
func (p *txPostings) Commit(ctx context.Context, posting entity.PostingData) error {
	if err := p.PostingRepository.Commit(ctx, posting); err != nil {
		return err
	}

	// Same-tx read: the row is visible here even before outer commit.
	cursor, err := p.CursorOf(ctx, posting.ID)
	if err != nil {
		return err
	}

	p.onCursor(cursor)

	return nil
}

// setCursor records the latest posting cursor of this transaction.
func (t *tx) setCursor(cursor string) {
	if cursor != "" {
		t.cursor = cursor
	}
}

// Postings returns the tx-scoped posting path.
func (t *tx) Postings() repository.PostingRepository { return t.postings }

// Holds returns the tx-scoped hold path.
func (t *tx) Holds() repository.HoldRepository { return t.holds }

// Idempotency returns the tx-scoped idempotency store.
func (t *tx) Idempotency() appport.IdempotencyStore { return t.idempotency }

// Outbox stages facts committed atomically with the writes.
func (t *tx) Outbox() appport.EventOutbox { return t.outbox }

// Cursor returns the ledger cursor of this transaction.
func (t *tx) Cursor() string { return t.cursor }

// idempotencyStore is the tx-scoped durable idempotency result store.
// Keys scope by (TenantID, Key): one tenant's keys never collide. The tenant
// of the last Reserve is remembered so Complete scopes to it as well.
type idempotencyStore struct {
	db     *gorm.DB
	tenant valueobject.TenantID
}

// Reserve leases Key for Fingerprint with replay/conflict semantics.
func (s *idempotencyStore) Reserve(ctx context.Context, rec appport.IdempotencyRecord) (appport.ReserveOutcome, error) {
	if rec.Key == "" {
		return appport.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_KEY_REQUIRED", "idempotency key is required")
	}

	if rec.TenantID.String() == "" {
		return appport.ReserveOutcome{}, entity.NewError("TENANT_REQUIRED", "tenant id is required")
	}

	s.tenant = rec.TenantID

	var existing models.IdempotencyModel

	err := s.db.WithContext(ctx).
		Where("tenant_id = ? AND key = ?", rec.TenantID.String(), rec.Key).
		First(&existing).Error

	switch {
	case err == nil:
		return replayOrConflict(existing, rec)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return s.lease(ctx, rec)
	default:
		return appport.ReserveOutcome{}, fmt.Errorf("postgres: reserve idempotency: %w", err)
	}
}

func replayOrConflict(existing models.IdempotencyModel, rec appport.IdempotencyRecord) (appport.ReserveOutcome, error) {
	if existing.Fingerprint != rec.Fingerprint {
		return appport.ReserveOutcome{}, entity.NewError("IDEMPOTENCY_CONFLICT", "idempotency key already used for a different request")
	}

	if existing.Response != nil {
		return appport.ReserveOutcome{Replay: true, Response: existing.Response}, nil
	}

	return appport.ReserveOutcome{}, nil
}

func (s *idempotencyStore) lease(ctx context.Context, rec appport.IdempotencyRecord) (appport.ReserveOutcome, error) {
	model := models.IdempotencyModel{
		TenantID:    rec.TenantID.String(),
		Key:         rec.Key,
		Fingerprint: rec.Fingerprint,
	}

	if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
		var existing models.IdempotencyModel

		// Lost the insert race: re-read and apply reserve rules once.
		readErr := s.db.WithContext(ctx).
			Where("tenant_id = ? AND key = ?", rec.TenantID.String(), rec.Key).
			First(&existing).Error
		if readErr != nil {
			return appport.ReserveOutcome{}, fmt.Errorf("postgres: lease idempotency: %w", err)
		}

		return replayOrConflict(existing, rec)
	}

	return appport.ReserveOutcome{}, nil
}

// Complete stores the command response for Key inside the same transaction,
// scoped to the Reserved tenant so identical keys in other tenants are safe.
func (s *idempotencyStore) Complete(ctx context.Context, key string, response []byte) error {
	query := s.db.WithContext(ctx).Model(&models.IdempotencyModel{}).Where("key = ?", key)
	if s.tenant.String() != "" {
		query = query.Where("tenant_id = ?", s.tenant.String())
	}

	res := query.Updates(map[string]any{"response": response, "completed_at": gorm.Expr("now()")})
	if res.Error != nil {
		return fmt.Errorf("postgres: complete idempotency: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return fmt.Errorf("postgres: complete idempotency: no lease for key")
	}

	return nil
}

// outboxStore stages facts inside the running transaction.
type outboxStore struct {
	db *gorm.DB
}

// Append stages facts; per-aggregate versions allocate monotonically inside
// the transaction so one aggregate may emit many facts (dedupe guard holds).
func (s *outboxStore) Append(ctx context.Context, facts ...appport.OutboxFact) error {
	offsets := make(map[string]int64)

	for _, fact := range facts {
		version, err := s.nextVersion(ctx, fact.AggregateID, offsets[fact.AggregateID])
		if err != nil {
			return err
		}

		offsets[fact.AggregateID]++

		model := models.OutboxModel{
			TenantID:         fact.TenantID.String(),
			LedgerID:         fact.LedgerID.String(),
			EventType:        fact.EventType,
			AggregateID:      fact.AggregateID,
			AggregateVersion: version,
			Payload:          fact.Payload,
			OccurredAt:       fact.OccurredAt,
		}

		if err := s.db.WithContext(ctx).Create(&model).Error; err != nil {
			return fmt.Errorf("postgres: append outbox: %w", err)
		}
	}

	return nil
}

// nextVersion allocates MAX(existing, 0) + 1 + already-staged offset.
func (s *outboxStore) nextVersion(ctx context.Context, aggregate string, offset int64) (int64, error) {
	var max int64

	err := s.db.WithContext(ctx).Model(&models.OutboxModel{}).
		Where("aggregate_id = ?", aggregate).
		Select("COALESCE(MAX(aggregate_version), 0)").Scan(&max).Error
	if err != nil {
		return 0, fmt.Errorf("postgres: outbox version: %w", err)
	}

	return max + 1 + offset, nil
}

// LockOrder returns account IDs in deterministic lock order (sorted).
// Transactions lock accounts in this order to prevent deadlocks.
func LockOrder(ids []valueobject.AccountID) []valueobject.AccountID {
	out := make([]valueobject.AccountID, len(ids))
	copy(out, ids)
	sort.Slice(out, func(i int, j int) bool { return out[i].String() < out[j].String() })

	return out
}

// AvailableMinor computes spendable minor units from posted entries minus
// ACTIVE holds for one account/asset. If normalSide is provided as DirectionCredit
// (e.g. Liability, Equity, Revenue per docs/ledger-core.md §5), credit increases
// balance and debit decreases balance. Otherwise, default normal side is Debit
// (Asset, Expense: debit increases balance, credit decreases balance).
// Overflow fails instead of wrapping.
func AvailableMinor(entries []entity.Entry, holds []entity.HoldData, normalSide ...valueobject.Direction) (int64, error) {
	isCreditNormal := len(normalSide) > 0 && normalSide[0] == valueobject.DirectionCredit

	total := int64(0)

	for _, entry := range entries {
		delta := entry.AmountMinor
		if isCreditNormal {
			if entry.Side == valueobject.DirectionDebit {
				delta = -delta
			}
		} else {
			if entry.Side == valueobject.DirectionCredit {
				delta = -delta
			}
		}

		sum, overflow := addChecked(total, delta)
		if overflow {
			return 0, fmt.Errorf("postgres: entry total overflow")
		}

		total = sum
	}

	for _, hold := range holds {
		if hold.State != entity.HoldActive {
			continue
		}

		remaining, overflow := addChecked(total, -hold.AmountMinor)
		if overflow {
			return 0, fmt.Errorf("postgres: hold total overflow")
		}

		total = remaining
	}

	return total, nil
}

// AvailableMinorForClass computes spendable minor units taking the account
// class (Asset/Expense vs Liability/Equity/Revenue) into account.
func AvailableMinorForClass(entries []entity.Entry, holds []entity.HoldData, class valueobject.AccountClass) (int64, error) {
	return AvailableMinor(entries, holds, class.NormalSide())
}

func addChecked(left int64, right int64) (int64, bool) {
	sum := left + right
	if (right > 0 && sum < left) || (right < 0 && sum > left) {
		return 0, true
	}

	return sum, false
}

// Compile-time port assertions.
var (
	_ appport.UnitOfWork       = (*UnitOfWork)(nil)
	_ appport.Tx               = (*tx)(nil)
	_ appport.IdempotencyStore = (*idempotencyStore)(nil)
	_ appport.EventOutbox      = (*outboxStore)(nil)
)
