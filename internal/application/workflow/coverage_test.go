package workflow_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/workflow"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestSagaUnknownStep(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)

	started, err := runner.Start(context.Background(), "t-1", "transfer", "saga-unknown", []workflow.Step{
		{Name: "a", MaxAttempts: 1, Run: func(context.Context) error { return nil }},
	})
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, started.State)

	renamed, err := runner.Resume(context.Background(), "t-1", "saga-unknown", []workflow.Step{
		{Name: "b", MaxAttempts: 1, Run: func(context.Context) error { return nil }},
	})
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, renamed.State)
}

func TestSagaResumeUnknownStepFails(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	ctx, cancel := context.WithCancel(context.Background())
	started, err := runner.Start(ctx, "t-1", "transfer", "saga-unknown-2", []workflow.Step{
		{
			Name: "a", MaxAttempts: 1,
			Run: func(context.Context) error {
				effects.apply("u2:a")
				cancel()
				return nil
			},
		},
		{Name: "b", MaxAttempts: 1, Run: func(context.Context) error { return nil }},
	})
	require.Equal(t, context.Canceled, err)

	resumed, err := runner.Resume(context.Background(), "t-1", "saga-unknown-2", []workflow.Step{
		{Name: "renamed", MaxAttempts: 1, Run: func(context.Context) error { return nil }},
	})
	assert.Equal(t, entity.NewError("SAGA_STEP_UNKNOWN", "saga step is not defined"), err)
	assert.Equal(t, workflow.SagaFailed, resumed.State)
	_ = started
}

func TestCompensationHelpers(t *testing.T) {
	t.Parallel()

	t.Run("no compensation is a no-op", func(t *testing.T) {
		assert.NoError(t, workflow.NoCompensation(context.Background()))
	})

	t.Run("compensation failure wraps cause", func(t *testing.T) {
		cause := errors.New("disk gone")
		wrapped := workflow.CompensationFailed("settle", cause)
		assert.Contains(t, wrapped.Error(), "settle")
		assert.Contains(t, wrapped.Error(), "disk gone")
	})

	t.Run("domain cause passes through unwrapped", func(t *testing.T) {
		cause := entity.NewError("HOLD_FAILED", "hold failed")
		assert.Equal(t, cause, workflow.CompensationFailed("settle", cause))
	})
}
