package lock

import "errors"

// Static error sentinels for err113; messages preserved.
var (
	ErrValkeyDown                             = errors.New("valkey down")
	ErrPaymentPostedButReconciliationDiverged = errors.New("payment posted but reconciliation diverged")
)
