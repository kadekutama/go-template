package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// AccrueDaily computes one day of interest under Actual/365: balance ×
// annualBPS / 10000 / 365 with checked math. Zero/negative balances accrue
// nothing (no posting). Direction depends on the account class and is
// returned for the posting template; the amount is always non-negative.
func AccrueDaily(balanceMinor, annualBPS int64, class valueobject.AccountClass) (amountMinor int64, debitNormal bool, err error) {
	if balanceMinor <= 0 || annualBPS <= 0 {
		return 0, false, nil
	}
	switch class {
	case valueobject.ClassAsset, valueobject.ClassExpense,
		valueobject.ClassLiability, valueobject.ClassEquity, valueobject.ClassRevenue:
	default:
		return 0, false, entity.NewError("ACCOUNT_CLASS_INVALID", "account class is invalid")
	}
	hi, ok := checkedMul(balanceMinor, annualBPS)
	if !ok {
		return 0, false, entity.NewError("INTEREST_OVERFLOW", "interest computation overflowed")
	}
	// Divide by 10000 (bps) then 365 (Actual/365). Integer truncation is the
	// declared rounding boundary; sub-minor-unit accrual posts nothing.
	amount := hi / 10000 / 365
	if amount <= 0 {
		return 0, false, nil
	}
	switch class {
	case valueobject.ClassAsset, valueobject.ClassExpense:
		return amount, true, nil
	default:
		return amount, false, nil
	}
}
