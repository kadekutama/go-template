package valueobject

import (
	"fmt"
)

// Direction is one side of double-entry: debit or credit. Neither universally
// means money in or out; the account class normal side decides display effect
// and posting templates decide legality.
type Direction string

// Journal sides.
const (
	DirectionDebit  Direction = "DEBIT"
	DirectionCredit Direction = "CREDIT"
)

// ParseDirection validates a journal side.
func ParseDirection(s string) (Direction, error) {
	switch Direction(s) {
	case DirectionDebit:
		return DirectionDebit, nil
	case DirectionCredit:
		return DirectionCredit, nil
	default:
		return "", fmt.Errorf("account: invalid direction %q", s)
	}
}

// AccountClass is the chart classification of an account.
type AccountClass string

// Chart account classes.
const (
	ClassAsset     AccountClass = "ASSET"
	ClassLiability AccountClass = "LIABILITY"
	ClassEquity    AccountClass = "EQUITY"
	ClassRevenue   AccountClass = "REVENUE"
	ClassExpense   AccountClass = "EXPENSE"
)

// ParseAccountClass validates a chart class.
func ParseAccountClass(s string) (AccountClass, error) {
	switch AccountClass(s) {
	case ClassAsset, ClassLiability, ClassEquity, ClassRevenue, ClassExpense:
		return AccountClass(s), nil
	default:
		return "", fmt.Errorf("account: invalid class %q", s)
	}
}

// NormalSide returns the display side per ledger-core §5. Both debit and
// credit remain legal on every class; templates restrict usage, not the class.
func (c AccountClass) NormalSide() Direction {
	switch c {
	case ClassAsset, ClassExpense:
		return DirectionDebit
	default:
		return DirectionCredit
	}
}

// AccountStatus is the lifecycle state of an account.
type AccountStatus string

// Account lifecycle states.
const (
	StatusActive AccountStatus = "ACTIVE"
	StatusFrozen AccountStatus = "FROZEN"
	StatusClosed AccountStatus = "CLOSED"
)

// ParseAccountStatus validates a lifecycle state.
func ParseAccountStatus(s string) (AccountStatus, error) {
	switch AccountStatus(s) {
	case StatusActive, StatusFrozen, StatusClosed:
		return AccountStatus(s), nil
	default:
		return "", fmt.Errorf("account: invalid status %q", s)
	}
}
