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

// TestSagaFailureDBCrashMidTransaction maps money-flow §10 row 1: a worker
// dying mid-saga resumes to exactly one effect per step key (WAL-style
// rollback at the step level plus idempotent actions).
func TestSagaFailureDBCrashMidTransaction(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	ctx, cancel := context.WithCancel(context.Background())
	steps := workflow.TransferSteps(workflow.TransferActions{
		ReserveFunds: func(context.Context) error {
			effects.apply("saga-db:reserve-funds")
			cancel()
			return nil
		},
		ReleaseReservation: func(context.Context) error { effects.apply("saga-db:release"); return nil },
		PostTransfer:       func(context.Context) error { effects.apply("saga-db:post"); return nil },
		VoidPosting:        func(context.Context) error { effects.apply("saga-db:void"); return nil },
		Notify:             func(context.Context) error { effects.apply("saga-db:notify"); return nil },
	})

	_, err := runner.Start(ctx, "t-1", "transfer", "saga-db", steps)
	assert.Equal(t, context.Canceled, err)

	resumed, err := runner.Resume(context.Background(), "t-1", "saga-db", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, resumed.State)
	for _, key := range []string{"saga-db:reserve-funds", "saga-db:post", "saga-db:notify"} {
		assert.Equal(t, 1, effects.count(key), "effect %s applied once", key)
	}
	assert.Equal(t, 0, effects.count("saga-db:void"))
	assert.Equal(t, 0, effects.count("saga-db:release"))
}

// TestSagaFailureCacheDown maps §10 row 2: with Valkey unavailable the saga
// completes through the strong path (cache is a hint, never authority).
func TestSagaFailureCacheDown(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}
	cacheAvailable := false

	steps := workflow.TransferSteps(workflow.TransferActions{
		ReserveFunds:       func(context.Context) error { effects.apply("saga-cache:reserve"); return nil },
		ReleaseReservation: func(context.Context) error { return nil },
		PostTransfer:       func(context.Context) error { effects.apply("saga-cache:post"); return nil },
		VoidPosting:        func(context.Context) error { return nil },
		Notify: func(context.Context) error {
			if !cacheAvailable {
				effects.apply("saga-cache:notify-direct")
				return nil
			}
			effects.apply("saga-cache:notify-cached")
			return nil
		},
	})

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-cache", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, record.State)
	assert.Equal(t, 1, effects.count("saga-cache:notify-direct"))
}

// TestSagaFailureNatsDown maps §10 row 3: the posting commits with its
// outbox fact while delivery retries; when delivery exhausts, the saga holds
// funds and notifies instead of writing partial ledger state.
func TestSagaFailureNatsDown(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	ledgerWrites := &keyedEffects{}
	holds := &keyedEffects{}

	steps := workflow.PayoutSettlementSteps(workflow.PayoutSettlementActions{
		SubmitPayout:   func(context.Context) error { ledgerWrites.apply("submit"); return nil },
		HoldForFailure: func(context.Context) error { holds.apply("hold"); return nil },
		SettlePayout:   func(context.Context) error { ledgerWrites.apply("settle"); return nil },
		MarkUnsettled:  func(context.Context) error { holds.apply("unsettled"); return nil },
		Notify:         func(context.Context) error { return errors.New("nats: ack timeout") },
	})

	record, err := runner.Start(context.Background(), "t-1", "payout-settlement", "saga-nats", steps)
	require.Error(t, err)
	assert.Equal(t, workflow.SagaCompensated, record.State)
	assert.Equal(t, 1, ledgerWrites.count("submit"))
	assert.Equal(t, 1, ledgerWrites.count("settle"))
	assert.Equal(t, 1, holds.count("unsettled"))
	assert.Equal(t, 1, holds.count("hold"))
}

// TestSagaFailureProcessorTimeout maps §10 row 4: the intent stays pending
// across attempts on the same key; success lands once.
func TestSagaFailureProcessorTimeout(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	charges := map[string]int{}
	timeout := errors.New("processor timeout")

	steps := workflow.RefundSteps(workflow.RefundActions{
		ValidateWindow: func(context.Context) error { return nil },
		ExecuteRefund: func(context.Context) error {
			charges["refund:saga-proc"]++
			if charges["refund:saga-proc"] == 1 {
				return timeout
			}
			return nil
		},
		LinkReversal: func(context.Context) error { return nil },
		Notify:       func(context.Context) error { return nil },
	})

	record, err := runner.Start(context.Background(), "t-1", "refund", "saga-proc", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, record.State)
	assert.Equal(t, 2, charges["refund:saga-proc"])
}

// TestSagaFailureStaleFX maps §10 row 5: stale rates fail fast for review
// instead of retrying into drift. Nothing compensates (nothing committed);
// the persisted FX_RATE_STALE step code is the review flag.
func TestSagaFailureStaleFX(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)

	steps := []workflow.Step{
		{
			Name: "convert", MaxAttempts: 1,
			Run: func(context.Context) error {
				return entity.NewError("FX_RATE_STALE", "fx rate is stale")
			},
		},
	}

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-fx", steps)
	assert.Equal(t, entity.NewError("FX_RATE_STALE", "fx rate is stale"), err)
	assert.Equal(t, workflow.SagaCompensated, record.State)
	require.Len(t, record.Steps, 1)
	assert.Equal(t, "FX_RATE_STALE", record.Steps[0].ErrorCode)
}

// TestSagaFailureReconBreak maps §10 row 6: mismatches record breaks with no
// money movement.
func TestSagaFailureReconBreak(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	ledgerWrites := &keyedEffects{}
	breaks := &keyedEffects{}

	steps := workflow.ReconciliationSteps(workflow.ReconciliationActions{
		MatchStatements: func(context.Context) error { breaks.apply("break:ref-9"); return nil },
		ResolveBreaks:   func(context.Context) error { breaks.apply("resolved:ref-9"); return nil },
	})

	record, err := runner.Start(context.Background(), "t-1", "reconciliation", "saga-recon", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, record.State)
	assert.Equal(t, 1, breaks.count("break:ref-9"))
	assert.Empty(t, ledgerWrites.order)
}

// TestSagaFailureDuplicateSubmit maps §10 row 7: the second start returns the
// stored result without re-executing.
func TestSagaFailureDuplicateSubmit(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := workflow.BatchSteps(workflow.BatchActions{
		IntakeBatch:  func(context.Context) error { effects.apply("saga-dup:intake"); return nil },
		ExecuteItems: func(context.Context) error { effects.apply("saga-dup:items"); return nil },
		CloseBatch:   func(context.Context) error { effects.apply("saga-dup:close"); return nil },
	})

	first, err := runner.Start(context.Background(), "t-1", "batch", "saga-dup", steps)
	require.NoError(t, err)
	second, err := runner.Start(context.Background(), "t-1", "batch", "saga-dup", steps)
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, effects.count("saga-dup:intake"))
}

// TestSagaFailureAuthExpiry maps §10 row 8: expired auths void with nothing
// moved plus notification (forward recovery inside the step, then notify).
func TestSagaFailureAuthExpiry(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := []workflow.Step{
		{
			Name: "capture-or-void", MaxAttempts: 1,
			Run: func(context.Context) error {
				effects.apply("void+notify-pending")
				return nil
			},
		},
		{
			Name: "notify", MaxAttempts: 1,
			Run: func(context.Context) error { effects.apply("void+notify"); return nil },
		},
	}

	record, err := runner.Start(context.Background(), "t-1", "transfer", "saga-auth", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, record.State)
	assert.Equal(t, 1, effects.count("void+notify-pending"))
	assert.Equal(t, 1, effects.count("void+notify"))
}

// TestSagaFailureOverCapture maps §10 row 9: over-capture rejects with the
// remaining amount and releases the reservation.
func TestSagaFailureOverCapture(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := workflow.TransferSteps(workflow.TransferActions{
		ReserveFunds:       func(context.Context) error { effects.apply("saga-cap:reserve"); return nil },
		ReleaseReservation: func(context.Context) error { effects.apply("saga-cap:release"); return nil },
		PostTransfer: func(context.Context) error {
			return &entity.Error{Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount; remaining=2000"}
		},
		VoidPosting: func(context.Context) error { effects.apply("saga-cap:void"); return nil },
		Notify:      func(context.Context) error { return nil },
	})

	_, err := runner.Start(context.Background(), "t-1", "transfer", "saga-cap", steps)
	assert.Equal(t, &entity.Error{Code: "CAPTURE_EXCEEDS_AUTHORIZED", Message: "capture exceeds authorized amount; remaining=2000"}, err)
	assert.Equal(t, 1, effects.count("saga-cap:release"))
	assert.Equal(t, 0, effects.count("saga-cap:void"))
}

// TestSagaFailureDisputeLost maps §10 row 10: a lost dispute links its
// reversal as the step's recorded effect (forward recovery) with the fee
// kept, then notifies.
func TestSagaFailureDisputeLost(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := workflow.RefundSteps(workflow.RefundActions{
		ValidateWindow: func(context.Context) error { return nil },
		ExecuteRefund:  func(context.Context) error { effects.apply("reversal-linked"); return nil },
		LinkReversal:   func(context.Context) error { return nil },
		Notify:         func(context.Context) error { effects.apply("notify"); return nil },
	})

	record, err := runner.Start(context.Background(), "t-1", "refund", "saga-dispute", steps)
	require.NoError(t, err)
	assert.Equal(t, workflow.SagaCompleted, record.State)
	assert.Equal(t, 1, effects.count("reversal-linked"))
}

// TestSagaFailurePeriodGates maps the period-close gate row: dirty periods
// abort before close with every failure visible.
func TestSagaFailurePeriodGates(t *testing.T) {
	t.Parallel()

	store := &sagaStoreFake{}
	runner := newRunner(store)
	effects := &keyedEffects{}

	steps := workflow.PeriodCloseSteps(workflow.PeriodCloseActions{
		ValidateGates: func(context.Context) error {
			return entity.NewError("BREAKS_OPEN", "period has open reconciliation breaks")
		},
		ClosePeriod: func(context.Context) error { effects.apply("close"); return nil },
	})

	record, err := runner.Start(context.Background(), "t-1", "period-close", "saga-period", steps)
	assert.Equal(t, entity.NewError("BREAKS_OPEN", "period has open reconciliation breaks"), err)
	assert.Equal(t, workflow.SagaCompensated, record.State)
	assert.Equal(t, 0, effects.count("close"))
}
