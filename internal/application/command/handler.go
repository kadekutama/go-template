// Package command owns the inbound command-handler contract shared by every
// application use case. Concrete commands live with their bounded context
// (E06-T02–T05/T11/T13); this package defines only the shape handlers take.
package command

import (
	"context"
)

// CommandHandler executes one validated command and returns its result with
// an ordinary Go return. A Result/monad wrapper is not part of the contract:
// errors are values carrying stable codes, translated at the edge.
type CommandHandler[C any, R any] interface {
	// Handle validates, authorizes, reserves idempotency, executes inside one
	// UnitOfWork, and completes the idempotency record. Identical resubmission
	// returns the original result without re-executing.
	Handle(ctx context.Context, cmd C) (R, error)
}
