// Package safe contains the panic-containment helper for background
// goroutines (E01-T06, SPEC §9.7).
//
// Go recovers a panic, reports value + stack to onPanic, and returns WITHOUT
// acknowledging work: the owner must treat the unit as failed/unknown (and, for
// financial mutations, retryable — never committed). Recovery is containment,
// not success.
package safe

import (
	"context"
	"fmt"
	"runtime/debug"
)

// Panic carries a recovered goroutine failure to its owner.
type Panic struct {
	Value any
	Stack []byte
}

func (p Panic) Error() string {
	return fmt.Sprintf("safe: recovered panic: %v", p.Value)
}

// Go runs fn in a new goroutine bound to ctx. A panic is recovered and
// delivered to onPanic exactly once; normal return delivers nothing.
// Callers MUST NOT treat onPanic delivery as completion.
func Go(ctx context.Context, onPanic func(Panic), fn func(ctx context.Context)) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				onPanic(Panic{Value: recovered, Stack: debug.Stack()})
			}
		}()
		fn(ctx)
	}()
}
