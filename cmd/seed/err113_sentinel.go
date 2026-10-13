package main

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrTestLoggerRequiresABuffer = errors.New("test logger requires a buffer")
	ErrSeedDATABASEURLIsRequired = errors.New("seed: DATABASE_URL is required")
)
