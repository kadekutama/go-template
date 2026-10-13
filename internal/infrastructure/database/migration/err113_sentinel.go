package migration

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrMigrationDBIsRequired                 = errors.New("migration: DB is required")
	ErrMigrationTimeoutMustBePositive        = errors.New("migration: timeout must be positive")
	ErrMigrationNoSQLMigrationFilesFound     = errors.New("migration: no .sql migration files found")
	ErrMigrationBadFileName                  = errors.New("migration: bad file name")
	ErrMigrationNeedsNonEmptyUpAndDownBlocks = errors.New("migration: needs non-empty Up and Down blocks")
	ErrMigrationDuplicateVersion             = errors.New("migration: duplicate version")
	ErrMigrationLegacyNumberingGaps          = errors.New("migration: legacy numbering gaps")
)
