package valueobject

import (
	"fmt"
	"time"
)

// PaymentMethod is the inbound/outbound rail for a payment.
type PaymentMethod string

// Payment rails.
const (
	MethodACH    PaymentMethod = "ACH"
	MethodWire   PaymentMethod = "WIRE"
	MethodRTP    PaymentMethod = "RTP"
	MethodCard   PaymentMethod = "CARD"
	MethodCrypto PaymentMethod = "CRYPTO"
	MethodWallet PaymentMethod = "WALLET"
)

// ParsePaymentMethod validates a payment rail.
func ParsePaymentMethod(s string) (PaymentMethod, error) {
	switch PaymentMethod(s) {
	case MethodACH, MethodWire, MethodRTP, MethodCard, MethodCrypto, MethodWallet:
		return PaymentMethod(s), nil
	default:
		return "", fmt.Errorf("payment: invalid method %q", s)
	}
}

// Capabilities describes what a rail can do.
type Capabilities struct {
	Push       bool
	Pull       bool
	Lag        time.Duration
	Reversible bool
}

// methodRegistry is the rail capability table. New rails extend the table,
// not a switch.
var methodRegistry = map[PaymentMethod]Capabilities{
	MethodACH:    {Push: true, Pull: true, Lag: 48 * time.Hour, Reversible: true},
	MethodWire:   {Push: true, Pull: false, Lag: 12 * time.Hour, Reversible: false},
	MethodRTP:    {Push: true, Pull: false, Lag: 15 * time.Minute, Reversible: false},
	MethodCard:   {Push: false, Pull: true, Lag: time.Hour, Reversible: true},
	MethodCrypto: {Push: true, Pull: false, Lag: time.Hour, Reversible: false},
	MethodWallet: {Push: true, Pull: true, Lag: time.Hour, Reversible: true},
}

// MethodCapabilities returns the registry entry per rail.
func MethodCapabilities(m PaymentMethod) (Capabilities, error) {
	caps, ok := methodRegistry[m]
	if !ok {
		return Capabilities{}, fmt.Errorf("payment: invalid method %q", string(m))
	}
	return caps, nil
}
