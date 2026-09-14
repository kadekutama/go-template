package specification

import (
	"context"
	"strconv"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// CurrencyCheck is one entry/account asset pair plus FX authorization.
type CurrencyCheck struct {
	EntryAsset   valueobject.AssetCode
	AccountAsset valueobject.AssetCode
	FXApproved   bool
}

// OriginalRef is a reference-existence candidate.
type OriginalRef struct {
	ID    string
	Found bool
}

// EntryAmountPositive passes for strictly positive minor-unit entries; side
// carries direction.
func EntryAmountPositive() Specification[entity.Entry] {
	return NewFuncSpec[entity.Entry]("INVALID_ENTRY_AMOUNT", "entry amount must be positive",
		func(_ context.Context, e entity.Entry) bool { return e.AmountMinor > 0 })
}

// PostingBalancesPerCurrency passes when, for every asset code, total debits
// equal total credits. Imbalanced assets report both figures in Details.
func PostingBalancesPerCurrency() Specification[entity.PostingData] {
	return evalFunc[entity.PostingData](func(_ context.Context, p entity.PostingData) SpecResult {
		type sums struct{ debit, credit int64 }
		byAsset := map[valueobject.AssetCode]*sums{}
		for _, e := range p.Entries {
			s, ok := byAsset[e.AssetCode]
			if !ok {
				s = &sums{}
				byAsset[e.AssetCode] = s
			}
			if e.Side == valueobject.DirectionDebit {
				s.debit += e.AmountMinor
			} else {
				s.credit += e.AmountMinor
			}
		}
		var out []Violation
		for asset, s := range byAsset {
			if s.debit != s.credit {
				out = append(out, Violation{
					Code:    "UNBALANCED_TRANSACTION",
					Message: "asset lot is unbalanced",
					Details: map[string]string{
						keyAsset:  string(asset),
						"debits":  strconv.FormatInt(s.debit, 10),
						"credits": strconv.FormatInt(s.credit, 10),
					},
				})
			}
		}
		if len(out) == 0 {
			return SpecResult{}
		}
		return SpecResult{Violations: out}
	})
}

// PostingTemplate is one versioned operation template: the operation name plus
// the allowed (class, side) rules.
type PostingTemplate struct {
	Operation string
	Version   string
	Rules     []TemplateRule
}

// TemplateRule permits one account class on one side.
type TemplateRule struct {
	Class valueobject.AccountClass
	Side  valueobject.Direction
}

// PostingTemplateAllowed passes when the posting operation matches the
// template and every entry account matches a (class, side) rule. Template and
// account data are copied at construction.
func PostingTemplateAllowed(template PostingTemplate, accounts map[valueobject.AccountID]entity.AccountData) Specification[entity.PostingData] {
	rules := make([]TemplateRule, len(template.Rules))
	copy(rules, template.Rules)
	accts := make(map[valueobject.AccountID]entity.AccountData, len(accounts))
	for k, v := range accounts {
		accts[k] = v
	}
	return evalFunc[entity.PostingData](func(_ context.Context, p entity.PostingData) SpecResult {
		fail := func(detail string) SpecResult {
			return SpecResult{Violations: []Violation{{
				Code: "INVALID_POSTING_TEMPLATE", Message: "posting does not match its operation template",
				Details: map[string]string{"operation": template.Operation, keyDetail: detail},
			}}}
		}
		if p.Operation != template.Operation {
			return fail("operation mismatch")
		}
		for _, e := range p.Entries {
			acct, ok := accts[e.AccountID]
			if !ok {
				return fail("unknown account " + e.AccountID.String())
			}
			matched := false
			for _, r := range rules {
				if r.Class == acct.Class && r.Side == e.Side {
					matched = true
					break
				}
			}
			if !matched {
				return fail("class/side not permitted for account " + e.AccountID.String())
			}
		}
		return SpecResult{}
	})
}

// ValidCurrency passes when both asset codes match or an approved FX rate
// covers the pair.
func ValidCurrency() Specification[CurrencyCheck] {
	return NewFuncSpec[CurrencyCheck]("CURRENCY_MISMATCH", "asset codes differ without approved FX",
		func(_ context.Context, c CurrencyCheck) bool {
			return c.EntryAsset != "" && c.EntryAsset == c.AccountAsset || c.FXApproved
		})
}

// OriginalExists passes when the referenced original was found.
func OriginalExists() Specification[OriginalRef] {
	return NewFuncSpec[OriginalRef]("ORIGINAL_NOT_FOUND", "referenced original does not exist",
		func(_ context.Context, r OriginalRef) bool { return r.Found })
}
