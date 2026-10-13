package valueobject

import (
	"fmt"
	"time"
)

// PayoutMethod is the outbound rail for a payout.
type PayoutMethod string

// Payout rails.
const (
	PayoutACH    PayoutMethod = "ACH"
	PayoutRTP    PayoutMethod = "RTP"
	PayoutFedNow PayoutMethod = "FEDNOW"
	PayoutWire   PayoutMethod = "WIRE"
	PayoutCheck  PayoutMethod = "CHECK"
)

// ParsePayoutMethod validates a payout rail.
func ParsePayoutMethod(s string) (PayoutMethod, error) {
	switch PayoutMethod(s) {
	case PayoutACH, PayoutRTP, PayoutFedNow, PayoutWire, PayoutCheck:
		return PayoutMethod(s), nil
	default:
		return "", fmt.Errorf("payout: invalid method %q: %w", s, ErrPayoutInvalidMethod)
	}
}

// payoutLags is the money-flow §5 expected settlement lag registry: RTP/FedNow
// minutes, ACH 1–2 business days (48h ceiling model), Wire same-day (12h
// model), Check 5 days. New rails extend the table, not a switch.
func payoutLags() map[PayoutMethod]time.Duration {
	return map[PayoutMethod]time.Duration{
		PayoutRTP:    payoutRTP15 * time.Minute,
		PayoutFedNow: payoutFedNow15 * time.Minute,
		PayoutACH:    payoutACH48 * time.Hour,
		PayoutWire:   payoutWire12 * time.Hour,
		PayoutCheck:  payoutCheck120 * time.Hour,
	}
}

// ExpectedSettlementLag returns the money-flow §5 expected settlement lag per
// rail.
func ExpectedSettlementLag(m PayoutMethod) (time.Duration, error) {
	lag, ok := payoutLags()[m]
	if !ok {
		return 0, fmt.Errorf("payout: invalid method %q: %w", string(m), ErrPayoutInvalidMethod2)
	}

	return lag, nil
}
