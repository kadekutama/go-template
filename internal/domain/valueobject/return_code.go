package valueobject

import (
	"fmt"
	"maps"
	"slices"
)

// ReturnDisposition is the handling outcome for a return/decline code.
type ReturnDisposition string

// Return dispositions.
const (
	ReturnAutoReversal ReturnDisposition = "AUTO_REVERSAL"
	ReturnManualReview ReturnDisposition = "MANUAL_REVIEW"
)

// returnCatalog maps ACH R-codes (subset) and card-decline codes to
// dispositions. Amount/authorization failures auto-reverse; fraud/account
// problems need human review.
var returnCatalog = map[string]ReturnDisposition{
	"R01": ReturnAutoReversal, "R02": ReturnAutoReversal, "R03": ReturnManualReview, "R04": ReturnManualReview,
	"R06": ReturnAutoReversal, "R07": ReturnManualReview, "R08": ReturnManualReview, "R09": ReturnAutoReversal,
	"R10": ReturnManualReview, "R11": ReturnManualReview, "R12": ReturnAutoReversal, "R13": ReturnManualReview,
	"R14": ReturnManualReview, "R15": ReturnManualReview, "R16": ReturnManualReview, "R17": ReturnManualReview,
	"CARD_DECLINED":        ReturnAutoReversal,
	"CARD_INSUFFICIENT":    ReturnAutoReversal,
	"CARD_FRAUD_SUSPECTED": ReturnManualReview,
	"CARD_EXPIRED":         ReturnManualReview,
	"CARD_INVALID":         ReturnManualReview,
}

// DispositionFor maps a return code to its disposition.
func DispositionFor(code string) (ReturnDisposition, error) {
	d, ok := returnCatalog[code]
	if !ok {
		return "", fmt.Errorf("payment: unknown return code %q", code)
	}
	return d, nil
}

// ReturnCodes returns every cataloged code in sorted order (for
// exhaustiveness tests).
func ReturnCodes() []string {
	return slices.Sorted(maps.Keys(returnCatalog))
}
