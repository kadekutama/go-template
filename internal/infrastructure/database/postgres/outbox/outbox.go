// Package outbox implements the transactional outbox relay (E07-T03):
// same-transaction envelope writes plus a SKIP LOCKED poller publishing
// through port.EventPublisher with at-least-once redelivery.
package outbox

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres/models"
)

// ClaimSQL claims one batch of undelivered facts with row-level locks that
// skip rows locked by peer pollers. Ordering is by id (insertion sequence);
// wall-clock order never defines delivery order. Expired claims are reclaimed
// after the lease timeout.
const ClaimSQL = `UPDATE outbox_events SET claimed_at = now()
WHERE id IN (
    SELECT id FROM outbox_events
    WHERE delivered_at IS NULL
      AND (claimed_at IS NULL OR claimed_at < now() - (? * INTERVAL '1 second'))
    ORDER BY id
    LIMIT ? FOR UPDATE SKIP LOCKED
) RETURNING id, tenant_id, ledger_id, event_type, aggregate_id, aggregate_version, payload, occurred_at`

// WriterParams carries constructor dependencies.
type WriterParams struct {
	DB *gorm.DB
}

// Writer persists envelope rows inside the caller's transaction.
type Writer struct {
	db *gorm.DB
}

// NewWriter builds the writer; DB must be non-nil.
func NewWriter(params WriterParams) (*Writer, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("outbox: writer needs DB")
	}

	return &Writer{db: params.DB}, nil
}

// AppendTx stages facts in tx (same commit as the state change), allocating
// per-aggregate versions monotonically (dedupe guard holds).
func (w *Writer) AppendTx(ctx context.Context, tx *gorm.DB, facts ...appport.OutboxFact) error {
	if tx == nil {
		return fmt.Errorf("outbox: transaction is required")
	}

	offsets := make(map[string]int64)

	for _, fact := range facts {
		var max int64

		err := tx.WithContext(ctx).Model(&models.OutboxModel{}).
			Where("aggregate_id = ?", fact.AggregateID).
			Select("COALESCE(MAX(aggregate_version), 0)").Scan(&max).Error
		if err != nil {
			return fmt.Errorf("outbox: version: %w", err)
		}

		offsets[fact.AggregateID]++

		model := models.OutboxModel{
			TenantID:         fact.TenantID.String(),
			LedgerID:         fact.LedgerID.String(),
			EventType:        fact.EventType,
			AggregateID:      fact.AggregateID,
			AggregateVersion: max + offsets[fact.AggregateID],
			Payload:          fact.Payload,
			OccurredAt:       fact.OccurredAt,
		}

		if err := tx.WithContext(ctx).Create(&model).Error; err != nil {
			return fmt.Errorf("outbox: append: %w", err)
		}
	}

	return nil
}

// DefaultMaxBatch bounds one claim batch.
const DefaultMaxBatch = 100

// DefaultBaseBackoff seeds jittered retry waits.
const DefaultBaseBackoff = 500 * time.Millisecond

// DefaultClaimTimeout is the duration a claimed batch remains locked before
// an undelivered event can be reclaimed by peer pollers.
const DefaultClaimTimeout = 60 * time.Second

// PollerParams carries poller dependencies.
type PollerParams struct {
	Publisher appport.EventPublisher
	// MaxBatch bounds one claim batch; 0 means DefaultMaxBatch.
	MaxBatch int
	// BaseBackoff seeds jittered retry waits; 0 means DefaultBaseBackoff.
	BaseBackoff time.Duration
	// ClaimTimeout is the lease duration for in-flight batches; 0 means DefaultClaimTimeout.
	ClaimTimeout time.Duration
	// Alert, when set, fires after each failed batch.
	Alert func(ctx context.Context, err error)
}

// Poller relays committed facts to the publisher.
type Poller struct {
	publisher    appport.EventPublisher
	maxBatch     int
	baseBackoff  time.Duration
	claimTimeout time.Duration
	alert        func(ctx context.Context, err error)
}

// NewPoller builds the poller; Publisher must be non-nil.
func NewPoller(params PollerParams) (*Poller, error) {
	if params.Publisher == nil {
		return nil, fmt.Errorf("outbox: poller needs a publisher")
	}

	maxBatch := params.MaxBatch
	if maxBatch <= 0 {
		maxBatch = DefaultMaxBatch
	}

	backoff := params.BaseBackoff
	if backoff <= 0 {
		backoff = DefaultBaseBackoff
	}

	claimTimeout := params.ClaimTimeout
	if claimTimeout <= 0 {
		claimTimeout = DefaultClaimTimeout
	}

	return &Poller{
		publisher:    params.Publisher,
		maxBatch:     maxBatch,
		baseBackoff:  backoff,
		claimTimeout: claimTimeout,
		alert:        params.Alert,
	}, nil
}

// MaxBatch reports the configured batch bound.
func (p *Poller) MaxBatch() int { return p.maxBatch }

// ClaimTimeout reports the configured lease timeout.
func (p *Poller) ClaimTimeout() time.Duration { return p.claimTimeout }

// Backoff returns the jittered wait for consecutive failure count n.
func (p *Poller) Backoff(n int) time.Duration {
	if n < 0 {
		n = 0
	}

	if n > 6 {
		n = 6
	}

	wait := p.baseBackoff * time.Duration(1<<n)
	jitter := time.Duration(rand.Int63n(int64(p.baseBackoff))) //nolint:gosec // jitter only

	return wait + jitter
}

// RunOnce claims one batch, publishes in aggregate order, and marks delivery.
// Crash between commit and publish leaves rows claimable after lease expiration.
func (p *Poller) RunOnce(ctx context.Context, db *gorm.DB) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("outbox: DB is required")
	}

	var rows []models.OutboxModel

	claimSec := int64(p.claimTimeout.Seconds())
	if claimSec <= 0 {
		claimSec = int64(DefaultClaimTimeout.Seconds())
	}

	if err := db.WithContext(ctx).Raw(ClaimSQL, claimSec, p.maxBatch).Scan(&rows).Error; err != nil {
		return 0, fmt.Errorf("outbox: claim: %w", err)
	}

	if len(rows) == 0 {
		return 0, nil
	}

	facts := make([]appport.OutboxFact, 0, len(rows))
	ids := make([]int64, 0, len(rows))

	for _, row := range rows {
		facts = append(facts, outboxToFact(row))
		ids = append(ids, row.ID)
	}

	if err := p.publisher.Publish(ctx, facts...); err != nil {
		if p.alert != nil {
			p.alert(ctx, err)
		}

		return 0, fmt.Errorf("outbox: publish: %w", err)
	}

	if err := db.WithContext(ctx).Model(&models.OutboxModel{}).
		Where("id IN ?", ids).Update("delivered_at", gorm.Expr("now()")).Error; err != nil {
		return 0, fmt.Errorf("outbox: mark delivered: %w", err)
	}

	return len(rows), nil
}

func outboxToFact(row models.OutboxModel) appport.OutboxFact {
	return appport.OutboxFact{
		TenantID:    valueobject.TenantID(row.TenantID),
		LedgerID:    valueobject.LedgerID(row.LedgerID),
		EventType:   row.EventType,
		AggregateID: row.AggregateID,
		Payload:     row.Payload,
		OccurredAt:  row.OccurredAt,
	}
}
