// Package query owns the inbound query-handler contract shared by every
// application read model. Concrete queries live with their bounded context;
// this package defines only the shape handlers take. Queries never perform
// business side effects.
package query

import (
	"context"
)

// Handler answers one read-model question with an ordinary Go return.
// Pagination uses opaque cursors owned by the adapter; strong reads name
// their consistency in the concrete contract.
type Handler[Q any, R any] interface {
	// Handle answers query without business side effects.
	Handle(ctx context.Context, query Q) (R, error)
}
