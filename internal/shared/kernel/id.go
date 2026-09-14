package kernel

import (
	"fmt"

	"github.com/google/uuid"
)

// IDGenerator mints unique identifiers (E01-T06, SPEC §7.10).
type IDGenerator interface {
	NewID() string
}

// UUIDGenerator mints UUIDv7 identifiers (ADR-012 owner directive, replacing
// ULID). v7 carries unix-millisecond time ordering; unlike ULID it is NOT
// monotonic within the same millisecond (random sub-millisecond bits), so
// callers MUST NOT rely on same-ms ordering — uniqueness is the contract.
type UUIDGenerator struct{}

// NewUUIDGenerator builds the default ID generator. No clock is needed:
// UUIDv7 reads system time internally.
func NewUUIDGenerator() *UUIDGenerator {
	return &UUIDGenerator{}
}

// NewID returns a new UUIDv7 string. It panics only if the OS randomness
// source fails (unrecoverable by definition).
func (g *UUIDGenerator) NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		panic(fmt.Sprintf("kernel: uuid v7: %v", err))
	}
	return id.String()
}
