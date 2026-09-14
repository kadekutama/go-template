package service

import (
	"math"
)

// checkedMul returns a*b and whether the multiplication fits int64. Callers
// map failure to their own stable error code.
func checkedMul(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	if a == math.MinInt64 && b == -1 || a == -1 && b == math.MinInt64 {
		return 0, false
	}
	hi := a * b
	if hi/b != a {
		return 0, false
	}
	return hi, true
}

// checkedAdd returns a+b and whether the addition fits int64.
func checkedAdd(a, b int64) (int64, bool) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < math.MinInt64-b) {
		return 0, false
	}
	return a + b, true
}
