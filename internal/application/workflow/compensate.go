package workflow

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// compensate runs completed steps in reverse order, marking each compensated.
// A compensation failure parks the saga FAILED with the error recorded: the
// saga never claims compensation it did not perform. Compensation writes
// hold/notify-style effects only — never partial ledger state.
func (r *Runner) compensate(ctx context.Context, record SagaRecord, steps []Step, failedIndex int, cause error) (SagaRecord, error) {
	byName := make(map[string]Step, len(steps))
	for _, step := range steps {
		byName[step.Name] = step
	}
	for i := failedIndex - 1; i >= 0; i-- {
		if record.Steps[i].State != StepCompleted {
			continue
		}
		step, ok := byName[record.Steps[i].Name]
		if !ok || step.Compensate == nil {
			record.Steps[i].State = StepSkipped
			continue
		}
		if err := step.Compensate(ctx); err != nil {
			record.Steps[i].State = StepFailed
			record.Steps[i].ErrorCode = errorCodeOf(err)
			record.State = SagaFailed
			record.UpdatedAt = r.clock.Now().UTC()
			if updateErr := r.store.UpdateSaga(ctx, record); updateErr != nil {
				return record, updateErr
			}
			return record, err
		}
		record.Steps[i].State = StepCompensated
		record.Steps[i].UpdatedAt = r.clock.Now().UTC()
	}
	record.State = SagaCompensated
	record.UpdatedAt = r.clock.Now().UTC()
	if err := r.store.UpdateSaga(ctx, record); err != nil {
		return record, err
	}
	return record, cause
}

// NoCompensation marks steps with nothing to undo (pure reads, validations,
// and notifications that carry their own dedupe).
func NoCompensation(_ context.Context) error { return nil }

// CompensationFailed builds the parked-saga error for failed compensation.
func CompensationFailed(step string, cause error) error {
	if domainErr, ok := cause.(*entity.Error); ok {
		return domainErr
	}
	return entity.Errorf("COMPENSATION_FAILED", "compensation failed at %s: %v", step, cause)
}
