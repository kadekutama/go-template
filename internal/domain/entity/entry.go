package entity

import (
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// Entry is one immutable side of double-entry: a positive integer minor-unit
// quantity on exactly one debit or credit side. Signed amounts and stored
// balance_after values are forbidden.
type Entry struct {
	ID          valueobject.EntryID
	PostingID   valueobject.PostingID
	AccountID   valueobject.AccountID
	Side        valueobject.Direction
	AmountMinor int64
	AssetCode   valueobject.AssetCode
	AccountSeq  int64
}

// Validate checks entry structure. Account existence, scope, asset match, and
// status are posting-construction checks (they need the account set), not
// entry checks.
func (e Entry) Validate() error {
	if e.ID.String() == "" {
		return NewError("ENTRY_ID_REQUIRED", "entry id is required")
	}
	if e.PostingID.String() == "" {
		return NewError("ENTRY_POSTING_REQUIRED", "posting id is required")
	}
	if e.AccountID.String() == "" {
		return NewError("ENTRY_ACCOUNT_REQUIRED", "account id is required")
	}
	if e.Side != valueobject.DirectionDebit && e.Side != valueobject.DirectionCredit {
		return NewError("ENTRY_SIDE_INVALID", "side must be DEBIT or CREDIT")
	}
	if e.AmountMinor <= 0 {
		return NewError("INVALID_ENTRY_AMOUNT", "entry amount must be a positive minor-unit quantity")
	}
	if e.AssetCode == "" {
		return NewError("ENTRY_ASSET_REQUIRED", "asset code is required")
	}
	if e.AccountSeq < 1 {
		return NewError("ENTRY_SEQUENCE_REQUIRED", "account sequence must be at least 1")
	}
	return nil
}
