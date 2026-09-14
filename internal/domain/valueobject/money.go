// Package valueobject holds immutable domain primitives: minor-unit money,
// the versioned asset registry contract, and type-safe identifiers.
package valueobject

import (
	"cmp"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
)

// Money is an immutable integer-minor-unit amount in one asset. Floats are
// forbidden: all arithmetic is checked int64 math that errors on overflow or
// cross-asset use instead of panicking.
type Money struct {
	amountMinor int64
	asset       AssetCode
}

// NewMoney validates and returns a Money value. Any int64 quantity is
// representable (corrections and negative availability exist downstream);
// entry positivity is enforced by the posting aggregate, not here.
func NewMoney(amountMinor int64, asset AssetCode) (Money, error) {
	if asset == "" {
		return Money{}, errors.New("money: asset code is required")
	}
	return Money{amountMinor: amountMinor, asset: asset}, nil
}

// MustMoney builds a Money value for tests with known-valid inputs. It panics
// only on programmer error (empty asset), never on domain input.
func MustMoney(amountMinor int64, asset AssetCode) Money {
	m, err := NewMoney(amountMinor, asset)
	if err != nil {
		panic(err)
	}
	return m
}

// AmountMinor returns the integer minor-unit quantity.
func (m Money) AmountMinor() int64 { return m.amountMinor }

// Asset returns the asset code of the amount.
func (m Money) Asset() AssetCode { return m.asset }

// IsZero reports whether the amount is exactly zero.
func (m Money) IsZero() bool { return m.amountMinor == 0 }

// IsPositive reports whether the amount is strictly positive.
func (m Money) IsPositive() bool { return m.amountMinor > 0 }

// checkAsset rejects cross-asset arithmetic with CURRENCY_MISMATCH.
func (m Money) checkAsset(o Money) error {
	if m.asset != o.asset {
		return fmt.Errorf("money: CURRENCY_MISMATCH %q vs %q", m.asset, o.asset)
	}
	return nil
}

func checkedAdd(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, errors.New("money: arithmetic overflow")
	}
	return a + b, nil
}

func checkedMul(a, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	if a == math.MinInt64 && b == -1 || a == -1 && b == math.MinInt64 {
		return 0, errors.New("money: arithmetic overflow")
	}
	hi := a * b
	if hi/b != a {
		return 0, errors.New("money: arithmetic overflow")
	}
	return hi, nil
}

// Add returns m+o for the same asset, rejecting overflow and cross-asset use.
func (m Money) Add(o Money) (Money, error) {
	if err := m.checkAsset(o); err != nil {
		return Money{}, err
	}
	sum, err := checkedAdd(m.amountMinor, o.amountMinor)
	if err != nil {
		return Money{}, err
	}
	return Money{amountMinor: sum, asset: m.asset}, nil
}

// Sub returns m-o for the same asset, rejecting overflow and cross-asset use.
func (m Money) Sub(o Money) (Money, error) {
	if err := m.checkAsset(o); err != nil {
		return Money{}, err
	}
	if o.amountMinor == math.MinInt64 {
		return Money{}, errors.New("money: arithmetic overflow")
	}
	diff, err := checkedAdd(m.amountMinor, -o.amountMinor)
	if err != nil {
		return Money{}, err
	}
	return Money{amountMinor: diff, asset: m.asset}, nil
}

// MulScalar returns m*k with overflow rejection.
func (m Money) MulScalar(k int64) (Money, error) {
	prod, err := checkedMul(m.amountMinor, k)
	if err != nil {
		return Money{}, err
	}
	return Money{amountMinor: prod, asset: m.asset}, nil
}

// DivScalar returns truncating integer division m/k. Division by zero errors.
// Remainders are discarded; callers needing exact splits use
// AllocateLargestRemainder.
func (m Money) DivScalar(k int64) (Money, error) {
	if k == 0 {
		return Money{}, errors.New("money: division by zero")
	}
	if m.amountMinor == math.MinInt64 && k == -1 {
		return Money{}, errors.New("money: arithmetic overflow")
	}
	return Money{amountMinor: m.amountMinor / k, asset: m.asset}, nil
}

// Compare orders same-asset amounts (-1, 0, +1). Cross-asset comparison
// errors with CURRENCY_MISMATCH instead of panicking.
func (m Money) Compare(o Money) (int, error) {
	if err := m.checkAsset(o); err != nil {
		return 0, err
	}
	switch {
	case m.amountMinor < o.amountMinor:
		return -1, nil
	case m.amountMinor > o.amountMinor:
		return 1, nil
	default:
		return 0, nil
	}
}

// MoneyDTO is the canonical minor-unit JSON shape owned by the domain:
// integer minor units plus asset code. The domain defines the shape (with
// tags) but never imports a JSON codec — byte marshaling belongs to outer
// layers via pkg/jsonparser (E01-T07 codec seam).
type MoneyDTO struct {
	AmountMinor int64     `json:"amount_minor"`
	AssetCode   AssetCode `json:"asset_code"`
}

// DTO projects Money onto its canonical JSON shape.
func (m Money) DTO() MoneyDTO {
	return MoneyDTO{AmountMinor: m.amountMinor, AssetCode: m.asset}
}

// MoneyFromDTO validates a decoded DTO. Float quantities are rejected by the
// codec before this point (non-integral JSON numbers do not decode into
// int64); empty asset codes error here.
func MoneyFromDTO(d MoneyDTO) (Money, error) {
	return NewMoney(d.AmountMinor, d.AssetCode)
}

// Format renders human-readable major units using the registry exponent
// (e.g. 1050 minor + exponent 2 → "10.50"). The registry is explicit: unknown
// codes error.
func (m Money) Format(reg Registry) (string, error) {
	info, err := reg.Lookup(m.asset)
	if err != nil {
		return "", err
	}
	// uint64 magnitude keeps MinInt64 renderable (negating it overflows int64).
	var mag uint64
	neg := false
	if m.amountMinor < 0 {
		neg = true
		mag = uint64(-(m.amountMinor + 1)) + 1 // #nosec G115 -- safe magnitude for negative int64
	} else {
		mag = uint64(m.amountMinor)
	}
	var scale uint64 = 1
	for i := 0; i < info.Exponent; i++ {
		scale *= 10
	}
	major, minor := mag/scale, mag%scale
	var sb strings.Builder
	if neg {
		sb.WriteByte('-')
	}
	fmt.Fprintf(&sb, "%d", major)
	if info.Exponent > 0 {
		fmt.Fprintf(&sb, ".%0*d", info.Exponent, minor)
	}
	return sb.String(), nil
}

func normalizeAllocationWeights(weights []int64) ([]int64, int64, error) {
	if len(weights) == 0 {
		return nil, 0, errors.New("money: allocation requires at least one weight")
	}
	var sum int64
	allZero := true
	for _, w := range weights {
		if w < 0 {
			return nil, 0, errors.New("money: allocation weights must not be negative")
		}
		if w != 0 {
			allZero = false
		}
		var err error
		sum, err = checkedAdd(sum, w)
		if err != nil {
			return nil, 0, err
		}
	}
	if allZero {
		eff := make([]int64, len(weights))
		for i := range eff {
			eff[i] = 1
		}
		return eff, int64(len(weights)), nil
	}
	return weights, sum, nil
}

type fractionalRemainder struct {
	index int
	rem   int64
}

// distributeRemainders awards 1 unit to the top fractional remainders (Hare-Niemeyer method).
func distributeRemainders(shares []int64, remainders []fractionalRemainder, count int64) {
	slices.SortFunc(remainders, func(a, b fractionalRemainder) int {
		if a.rem != b.rem {
			if b.rem > a.rem {
				return 1
			}
			return -1
		}
		return cmp.Compare(a.index, b.index)
	})
	for i := int64(0); i < count && int(i) < len(remainders); i++ {
		shares[remainders[i].index]++
	}
}

// AllocateLargestRemainder splits total into len(weights) shares summing
// exactly to total using the Hare-Niemeyer (Hamilton) method (money-flow §2.13):
// each share receives floor(absTotal * weight / sumWeights), and leftover units
// are awarded one each to the shares with the largest fractional remainders
// ((absTotal * weight) % sumWeights), with ties broken by lowest index.
// Negative totals scale symmetrically: each share matches the sign.
// math.MinInt64 or negative weights return an error.
func AllocateLargestRemainder(total int64, weights []int64) ([]int64, error) {
	if total == math.MinInt64 {
		return nil, errors.New("money: arithmetic overflow")
	}
	eff, sum, err := normalizeAllocationWeights(weights)
	if err != nil {
		return nil, err
	}
	absTotal := total
	negative := total < 0
	if negative {
		absTotal = -absTotal
	}
	shares := make([]int64, len(eff))
	remainders := make([]fractionalRemainder, len(eff))
	var assigned int64
	for i, w := range eff {
		prod, err := checkedMul(absTotal, w)
		if err != nil {
			return nil, err
		}
		quotient := prod / sum
		shares[i] = quotient
		assigned += quotient
		remainders[i] = fractionalRemainder{index: i, rem: prod % sum}
	}
	leftover := absTotal - assigned
	distributeRemainders(shares, remainders, leftover)
	if negative {
		for i := range shares {
			shares[i] = -shares[i]
		}
	}
	return shares, nil
}
