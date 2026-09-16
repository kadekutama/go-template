package port

import (
	"time"
)

// Clock is the injectable wall-clock boundary. Application code reads time
// only through this port so tests are deterministic and UnitOfWork callbacks
// stay retry-safe.
type Clock interface {
	// Now returns the current UTC time.
	Now() time.Time
}
