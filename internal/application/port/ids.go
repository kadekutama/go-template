package port

import (
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// IDGenerator mints new identities for commands. It aliases the domain
// contract so there is exactly one identity shape; application code accepts
// it as an explicit parameter wherever new IDs are needed and never hides
// generation behind package state.
type IDGenerator = valueobject.IDGenerator
