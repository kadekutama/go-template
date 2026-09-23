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
	"sync"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

// Panic carries a recovered goroutine failure to its owner.
type Panic struct {
	Value any
	Stack []byte
}

func (p Panic) Error() string {
	return fmt.Sprintf("safe: recovered panic: %v", p.Value)
}

// Go runs fn in a new goroutine bound to ctx. A panic is recovered, logged
// with stack trace via logger (if non-nil) using standardized error and metadata
// keys, and delivered to onPanic (if non-nil). Normal return delivers nothing.
// Callers MUST NOT treat onPanic delivery or recovery as completion.
func Go(ctx context.Context, logger log.Logger, onPanic func(Panic), fn func(ctx context.Context)) {
	if fn == nil {
		return
	}
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				p := Panic{Value: recovered, Stack: debug.Stack()}
				if logger != nil {
					logger.Error(ctx, "recovered panic in goroutine",
						log.Err(p),
						log.Metadata(map[string]any{
							"stack": string(p.Stack),
						}),
					)
				}
				if onPanic != nil {
					onPanic(p)
				}
			}
		}()
		fn(ctx)
	}()
}

// Group is a panic-containing goroutine group built on sync.WaitGroup.Go
// (Go 1.25+): Wait blocks until every spawned goroutine has returned, and
// each goroutine's panic is contained exactly as in Go (logged with stack
// and delivered to onPanic). The zero value is ready for use; a Group must
// not be copied after first use.
type Group struct {
	wg      sync.WaitGroup
	logger  log.Logger
	onPanic func(Panic)
}

// NewGroup builds a Group that reports contained panics through the given
// logger and onPanic sink (either may be nil).
func NewGroup(logger log.Logger, onPanic func(Panic)) *Group {
	return &Group{logger: logger, onPanic: onPanic}
}

// Go runs fn in a new goroutine tracked by the group, bound to ctx. Panics
// are contained; delivery of a Panic does NOT count as completion.
func (g *Group) Go(ctx context.Context, fn func(ctx context.Context)) {
	if g == nil || fn == nil {
		return
	}

	g.wg.Go(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				p := Panic{Value: recovered, Stack: debug.Stack()}
				if g.logger != nil {
					g.logger.Error(ctx, "recovered panic in goroutine",
						log.Err(p),
						log.Metadata(map[string]any{
							"stack": string(p.Stack),
						}),
					)
				}
				if g.onPanic != nil {
					g.onPanic(p)
				}
			}
		}()
		fn(ctx)
	})
}

// Wait blocks until all goroutines spawned through this group have returned.
func (g *Group) Wait() {
	if g == nil {
		return
	}

	g.wg.Wait()
}
