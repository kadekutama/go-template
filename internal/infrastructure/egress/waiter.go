package egress

import "context"

// Waiter admits one outbound provider operation. Adapters consume this
// interface so tests can prove permit-per-attempt behavior without Docker,
// while production wires *Limiter. Every actual attempt, including retries,
// must acquire exactly one permit before any provider I/O.
type Waiter interface {
	Wait(ctx context.Context, provider, operation string) error
}
