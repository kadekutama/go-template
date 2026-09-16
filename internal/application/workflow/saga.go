// Package workflow owns saga orchestration for long-running money flows:
// persisted step states, idempotent actions, reverse-order compensation, and
// crash resume. Workers (E14) drive Resume; the schema lands in E07-T10.
// Actions behind steps stay idempotent on sagaID:step keys; the runner
// guarantees ordering, timeouts, retries, and compensation discipline.
package workflow

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// Saga lifecycle states.
const (
	SagaRunning     = "RUNNING"
	SagaCompleted   = "COMPLETED"
	SagaCompensated = "COMPENSATED"
	SagaFailed      = "FAILED"
)

// Step lifecycle states.
const (
	StepPending     = "PENDING"
	StepRunning     = "RUNNING"
	StepCompleted   = "COMPLETED"
	StepFailed      = "FAILED"
	StepCompensated = "COMPENSATED"
	StepSkipped     = "SKIPPED"
)

// StepNotify is the shared notification step name across saga kinds.
const StepNotify = "notify"

// StepRecord is the durable state of one saga step.
type StepRecord struct {
	Name      string
	State     string
	Attempts  int
	ErrorCode string
	UpdatedAt time.Time
}

// SagaRecord is the durable saga: identity, kind, overall state, and every
// step state. Records are the resume contract for crashed workers.
type SagaRecord struct {
	ID        string
	TenantID  string
	Kind      string
	State     string
	Steps     []StepRecord
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SagaStore is the consumer-owned saga persistence boundary. The schema lands
// in E07-T10; this interface is the contract it implements.
type SagaStore interface {
	// CreateSaga persists one RUNNING saga. Strong write; fails on duplicate ID.
	CreateSaga(ctx context.Context, record SagaRecord) error
	// FindSaga returns one saga by tenant + ID. Strong read.
	FindSaga(ctx context.Context, tenantID, sagaID string) (SagaRecord, error)
	// UpdateSaga replaces one saga record. Strong write.
	UpdateSaga(ctx context.Context, record SagaRecord) error
}

// Step is one idempotent saga action with its compensation. Run effects key
// on sagaID:stepName; MaxAttempts bounds retries (1 = fail fast); Timeout
// bounds each attempt via ctx; a nil Compensate means nothing to undo.
type Step struct {
	Name        string
	MaxAttempts int
	Timeout     time.Duration
	Run         func(ctx context.Context) error
	Compensate  func(ctx context.Context) error
}

// RunnerParams encapsulates dependencies for Runner.
type RunnerParams struct {
	Store SagaStore
	Clock port.Clock
}

// Runner executes sagas with persisted step states. Saga identities arrive
// explicit at Start (idempotency keys); the runner mints nothing.
type Runner struct {
	store SagaStore
	clock port.Clock
}

// NewRunner creates an encapsulated Runner with validated dependencies.
func NewRunner(params RunnerParams) *Runner {
	return &Runner{
		store: params.Store,
		clock: params.Clock,
	}
}

// Start creates a RUNNING saga (or returns the stored record when the saga
// ID already exists: duplicate starts never re-execute) and runs it.
func (r *Runner) Start(ctx context.Context, tenantID, kind, sagaID string, steps []Step) (SagaRecord, error) {
	if strings.TrimSpace(sagaID) == "" {
		return SagaRecord{}, entity.NewError("SAGA_ID_REQUIRED", "saga id is required")
	}
	if len(steps) == 0 {
		return SagaRecord{}, entity.NewError("SAGA_STEPS_REQUIRED", "saga requires at least one step")
	}
	existing, err := r.store.FindSaga(ctx, tenantID, sagaID)
	if err == nil {
		return existing, nil
	}
	now := r.clock.Now().UTC()
	record := SagaRecord{
		ID: sagaID, TenantID: tenantID, Kind: kind, State: SagaRunning,
		CreatedAt: now, UpdatedAt: now,
	}
	for _, step := range steps {
		record.Steps = append(record.Steps, StepRecord{Name: step.Name, State: StepPending, UpdatedAt: now})
	}
	if err := r.store.CreateSaga(ctx, record); err != nil {
		return SagaRecord{}, err
	}
	return r.drive(ctx, record, steps)
}

// Resume reloads a saga and continues from its first incomplete step.
// Crashed workers resume here; COMPLETED steps are skipped, never re-run.
func (r *Runner) Resume(ctx context.Context, tenantID, sagaID string, steps []Step) (SagaRecord, error) {
	record, err := r.store.FindSaga(ctx, tenantID, sagaID)
	if err != nil {
		return SagaRecord{}, err
	}
	if record.State == SagaCompleted || record.State == SagaCompensated {
		return record, nil
	}
	return r.drive(ctx, record, steps)
}

// drive runs pending steps in order, compensating in reverse on exhaustion.
// A canceled context abandons the run WITHOUT compensating (crash): the
// persisted record keeps completed steps for Resume.
func (r *Runner) drive(ctx context.Context, record SagaRecord, steps []Step) (SagaRecord, error) {
	byName := make(map[string]Step, len(steps))
	for _, step := range steps {
		byName[step.Name] = step
	}
	for i := range record.Steps {
		if err := ctx.Err(); err != nil {
			return record, err
		}
		if record.Steps[i].State == StepCompleted || record.Steps[i].State == StepCompensated || record.Steps[i].State == StepSkipped {
			continue
		}
		step, ok := byName[record.Steps[i].Name]
		if !ok {
			return r.fail(ctx, record, i, "SAGA_STEP_UNKNOWN", "saga step is not defined")
		}
		if err := r.runStep(ctx, &record, i, step); err != nil {
			// Parent cancellation abandons without compensating (crash):
			// attempt timeouts arrive as step errors with a live parent.
			if ctx.Err() != nil {
				return record, ctx.Err()
			}
			return r.compensate(ctx, record, steps, i, err)
		}
	}
	record.State = SagaCompleted
	record.UpdatedAt = r.clock.Now().UTC()
	if err := r.store.UpdateSaga(ctx, record); err != nil {
		return record, err
	}
	return record, nil
}

// runStep attempts one step up to MaxAttempts with per-attempt timeouts,
// persisting attempts as they happen. A canceled context stops retrying and
// returns the cancellation without compensating (crash path: Resume
// continues later).
func (r *Runner) runStep(ctx context.Context, record *SagaRecord, index int, step Step) error {
	maxAttempts := max(step.MaxAttempts, 1)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		record.Steps[index].State = StepRunning
		record.Steps[index].Attempts++
		record.UpdatedAt = r.clock.Now().UTC()
		if err := r.store.UpdateSaga(ctx, *record); err != nil {
			return err
		}
		attemptCtx := ctx
		cancel := func() {}
		if step.Timeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, step.Timeout)
		}
		runErr := step.Run(attemptCtx)
		cancel()
		if runErr == nil {
			record.Steps[index].State = StepCompleted
			record.Steps[index].UpdatedAt = r.clock.Now().UTC()
			if err := r.store.UpdateSaga(ctx, *record); err != nil {
				return err
			}
			return nil
		}
		record.Steps[index].ErrorCode = errorCodeOf(runErr)
		if record.Steps[index].Attempts >= maxAttempts {
			record.Steps[index].State = StepFailed
			record.UpdatedAt = r.clock.Now().UTC()
			if err := r.store.UpdateSaga(ctx, *record); err != nil {
				return err
			}
			return runErr
		}
	}
}

// fail parks a saga FAILED when its definition cannot proceed.
func (r *Runner) fail(ctx context.Context, record SagaRecord, index int, code, message string) (SagaRecord, error) {
	record.Steps[index].State = StepFailed
	record.Steps[index].ErrorCode = code
	record.State = SagaFailed
	record.UpdatedAt = r.clock.Now().UTC()
	if err := r.store.UpdateSaga(ctx, record); err != nil {
		return record, err
	}
	return record, entity.NewError(code, message)
}

// errorCodeOf extracts the stable code for step failure records,
// unwrapping joined/wrapped domain errors.
func errorCodeOf(err error) string {
	var domainErr *entity.Error
	if errors.As(err, &domainErr) {
		return domainErr.Code
	}
	return "STEP_FAILED"
}
