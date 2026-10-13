package seed

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrSeedLedgerAccountAndPostingStoresAreRequired = errors.New("seed: ledger, account, and posting stores are required")
	ErrPostgresSeedDBIsRequired                     = errors.New("postgres seed: DB is required")
	ErrSeedPostingNeedsAtLeastTwoLines              = errors.New("seed: posting needs at least two lines")
)
