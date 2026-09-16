package workflow

import (
	"context"
	"time"
)

// TransferActions carries the transfer saga effects. Every Run effect keys
// on sagaID:stepName for idempotent resume; compensations undo committed
// steps in reverse order.
type TransferActions struct {
	ReserveFunds       func(ctx context.Context) error
	ReleaseReservation func(ctx context.Context) error
	PostTransfer       func(ctx context.Context) error
	VoidPosting        func(ctx context.Context) error
	Notify             func(ctx context.Context) error
}

// TransferSteps builds the transfer saga: reserve → post → notify. A failed
// post releases the reservation; a failed notify voids the posting so money
// never rests unannounced.
func TransferSteps(actions TransferActions) []Step {
	return []Step{
		{Name: "reserve-funds", MaxAttempts: 3, Timeout: 10 * time.Second, Run: actions.ReserveFunds, Compensate: actions.ReleaseReservation},
		{Name: "post-transfer", MaxAttempts: 3, Timeout: 10 * time.Second, Run: actions.PostTransfer, Compensate: actions.VoidPosting},
		{Name: StepNotify, MaxAttempts: 5, Timeout: 10 * time.Second, Run: actions.Notify},
	}
}
