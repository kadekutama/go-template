package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// ValidateRefundToOriginalMethod rejects refund-to-original-method on
// irreversible rails with a stable code.
func ValidateRefundToOriginalMethod(m valueobject.PaymentMethod) error {
	caps, err := valueobject.MethodCapabilities(m)
	if err != nil {
		return entity.NewError("PAYMENT_METHOD_UNKNOWN", "payment method is unknown")
	}
	if !caps.Reversible {
		return entity.NewError("METHOD_IRREVERSIBLE", "payment method does not support refund to original method")
	}
	return nil
}

// TransitionPayment enforces the payment workflow machine.
func TransitionPayment(from, to valueobject.PaymentStatus) (valueobject.PaymentStatus, error) {
	if !valueobject.CanTransitionPayment(from, to) {
		return from, entity.NewError("PAYMENT_TRANSITION_ILLEGAL", "illegal payment transition")
	}
	return to, nil
}

// DispositionFor maps a return code to auto-reversal or manual review.
func DispositionFor(code string) (valueobject.ReturnDisposition, error) {
	d, err := valueobject.DispositionFor(code)
	if err != nil {
		return "", entity.NewError("RETURN_CODE_UNKNOWN", "return code is unknown")
	}
	return d, nil
}

// AddLink enforces uniqueness per (payment, linked) pair against the
// caller-supplied existing set.
func AddLink(existing map[string]entity.PaymentLink, link entity.PaymentLink) error {
	if err := link.Validate(); err != nil {
		return err
	}
	if _, dup := existing[link.LinkKey()]; dup {
		return entity.NewError("DUPLICATE_LINK", "payment link already exists")
	}
	return nil
}

// ProviderOutcome is the resolution state after a provider call.
type ProviderOutcome string

// Provider outcomes.
const (
	OutcomeConfirmed ProviderOutcome = "CONFIRMED"
	OutcomeUnknown   ProviderOutcome = "OUTCOME_UNKNOWN"
	OutcomeFailed    ProviderOutcome = "FAILED"
)

// ResolveTimeout maps a provider timeout to OUTCOME_UNKNOWN: the caller MUST
// query provider state with the idempotency key before any money-moving retry.
func ResolveTimeout() ProviderOutcome {
	return OutcomeUnknown
}

// RequireStatusLookup reports whether a money-moving retry is allowed: never
// directly from UNKNOWN without a confirmed lookup.
func RequireStatusLookup(outcome ProviderOutcome, lookupConfirmed bool) error {
	if outcome == OutcomeUnknown && !lookupConfirmed {
		return entity.NewError("OUTCOME_UNKNOWN", "provider outcome unknown: status lookup required before retry")
	}
	return nil
}

// DuplicateDelivery reports whether a provider delivery is a replay: replays
// return the existing link state without another posting.
func DuplicateDelivery(existing map[string]entity.ProviderObjectLink, link entity.ProviderObjectLink) (entity.ProviderObjectLink, bool, error) {
	if err := link.Validate(); err != nil {
		return entity.ProviderObjectLink{}, false, err
	}
	if prev, ok := existing[link.DeliveryKey()]; ok {
		return prev, true, nil
	}
	return link, false, nil
}
