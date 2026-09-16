package workflow_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/workflow"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

type sagaStoreFake struct {
	mu      sync.Mutex
	records map[string]workflow.SagaRecord
}

func (s *sagaStoreFake) CreateSaga(_ context.Context, record workflow.SagaRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.records == nil {
		s.records = map[string]workflow.SagaRecord{}
	}
	if _, dup := s.records[record.ID]; dup {
		return entity.NewError("SAGA_CONFLICT", "saga id already exists")
	}
	s.records[record.ID] = record
	return nil
}

func (s *sagaStoreFake) FindSaga(_ context.Context, _ string, sagaID string) (workflow.SagaRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[sagaID]
	if !ok {
		return workflow.SagaRecord{}, entity.NewError("SAGA_NOT_FOUND", "saga is unknown")
	}
	return record, nil
}

func (s *sagaStoreFake) UpdateSaga(_ context.Context, record workflow.SagaRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.ID] = record
	return nil
}

type sagaClock struct{}

func (sagaClock) Now() time.Time { return time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC) }

func newRunner(store *sagaStoreFake) *workflow.Runner {
	return workflow.NewRunner(workflow.RunnerParams{Store: store, Clock: sagaClock{}})
}

// keyedEffects models idempotent step effects: each key applies once no
// matter how often the action runs (the contract sagas rely on for resume).
type keyedEffects struct {
	mu      sync.Mutex
	applied map[string]int
	order   []string
}

func (e *keyedEffects) apply(key string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.applied == nil {
		e.applied = map[string]int{}
	}
	if e.applied[key] > 0 {
		return
	}
	e.applied[key] = 1
	e.order = append(e.order, key)
}

func (e *keyedEffects) count(key string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.applied[key]
}

func TestSagaCrashResume(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	ctx, cancel := context.WithCancel(context.Background())
	steps := []workflow.Step{
		{
			Name: "a", MaxAttempts: 1,
			Run: func(context.Context) error {
				effects.apply("saga-1:a")
				cancel()
				return nil
			},
		},
		{Name: "b", MaxAttempts: 1, Run: func(context.Context) error {
			effects.apply("saga-1:b")
			return nil
		}},
		{Name: "c", MaxAttempts: 1, Run: func(context.Context) error {
			effects.apply("saga-1:c")
			return nil
		}},
	}

	_, err := runner.Start(ctx, "t-1", "transfer", "saga-1", steps)
	assert.Equal(t, context.Canceled, err)

	resumed, err := runner.Resume(context.Background(), "t-1", "saga-1", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, resumed.State)
	assert.Equal(t, 1, effects.count("saga-1:a"))
	assert.Equal(t, 1, effects.count("saga-1:b"))
	assert.Equal(t, 1, effects.count("saga-1:c"))
}

func TestSagaCompensationOrder(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := []workflow.Step{
		{
			Name: "a", MaxAttempts: 1,
			Run:        func(context.Context) error { effects.apply("run:a"); return nil },
			Compensate: func(context.Context) error { effects.apply("undo:a"); return nil },
		},
		{
			Name: "b", MaxAttempts: 1,
			Run:        func(context.Context) error { effects.apply("run:b"); return nil },
			Compensate: func(context.Context) error { effects.apply("undo:b"); return nil },
		},
		{
			Name: "c", MaxAttempts: 1,
			Run: func(context.Context) error { return entity.NewError("STEP_BOOM", "step c failed") },
		},
	}

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-2", steps)
	assert.Equal(t, entity.NewError("STEP_BOOM", "step c failed"), err)
	assert.Equal(t, workflow.SagaCompensated, record.State)
	assert.Equal(t, []string{"run:a", "run:b", "undo:b", "undo:a"}, effects.order)
}

func TestSagaCompensationFailure(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)

	steps := []workflow.Step{
		{
			Name: "a", MaxAttempts: 1,
			Run:        func(context.Context) error { return nil },
			Compensate: func(context.Context) error { return errors.New("undo exploded") },
		},
		{
			Name: "b", MaxAttempts: 1,
			Run: func(context.Context) error { return entity.NewError("STEP_BOOM", "step b failed") },
		},
	}

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-3", steps)
	assert.Equal(t, errors.New("undo exploded"), err)
	assert.Equal(t, workflow.SagaFailed, record.State)
}

func TestSagaDuplicateStart(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := []workflow.Step{
		{Name: "a", MaxAttempts: 1, Run: func(context.Context) error {
			effects.apply("saga-4:a")
			return nil
		}},
	}

	first, err := runner.Start(context.Background(), "t-1", "transfer", "saga-4", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, first.State)

	second, err := runner.Start(context.Background(), "t-1", "transfer", "saga-4", steps)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, effects.count("saga-4:a"))
}

func TestSagaStepTimeout(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)

	steps := []workflow.Step{
		{
			Name: "slow", MaxAttempts: 1, Timeout: time.Millisecond,
			Run: func(ctx context.Context) error {
				select {
				case <-time.After(50 * time.Millisecond):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		},
	}

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-5", steps)
	assert.Error(t, err)
	assert.Equal(t, workflow.SagaCompensated, record.State)
}

func TestSagaResumeTerminal(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	runs := 0

	steps := []workflow.Step{
		{Name: "a", MaxAttempts: 1, Run: func(context.Context) error {
			runs++
			return nil
		}},
	}

	_, err := runner.Start(context.Background(), "t-1", "transfer", "saga-6", steps)
	require.NoError(t, err)
	_, err = runner.Resume(context.Background(), "t-1", "saga-6", steps)
	require.NoError(t, err)
	assert.Equal(t, 1, runs)
}

func TestSagaValidation(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)

	t.Run("empty id rejected", func(t *testing.T) {
		_, err := runner.Start(context.Background(), "t-1", "transfer", "", []workflow.Step{
			{Name: "a", Run: func(context.Context) error { return nil }},
		})
		assert.Equal(t, entity.NewError("SAGA_ID_REQUIRED", "saga id is required"), err)
	})

	t.Run("no steps rejected", func(t *testing.T) {
		_, err := runner.Start(context.Background(), "t-1", "transfer", "saga-7", nil)
		assert.Equal(t, entity.NewError("SAGA_STEPS_REQUIRED", "saga requires at least one step"), err)
	})
}

var (
	_ port.Clock = sagaClock{}
)
