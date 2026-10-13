package persistence

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrInsufficientAvailableBalance                 = errors.New("insufficient available balance")
	ErrSimulatedCrashBeforeCommit                   = errors.New("simulated crash before commit")
	ErrChangedFingerprintMustConflictNeverReExecute = errors.New("changed fingerprint must conflict, never re-execute")
	ErrPostgresSeedDBIsRequired                     = errors.New("postgres seed: DB is required")
)
