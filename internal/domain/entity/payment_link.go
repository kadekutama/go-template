package entity

// PaymentLink binds a payment to an invoice/order/subscription. Uniqueness is
// per (payment, linked-type, linked-id) pair.
type PaymentLink struct {
	PaymentID  string
	LinkedType string
	LinkedID   string
}

// Validate checks link identity.
func (l PaymentLink) Validate() error {
	if l.PaymentID == "" {
		return NewError("PAYMENT_ID_REQUIRED", "payment link requires a payment id")
	}
	if l.LinkedType == "" || l.LinkedID == "" {
		return NewError("LINK_TARGET_REQUIRED", "payment link requires a linked type and id")
	}
	switch l.LinkedType {
	case "invoice", "order", "subscription":
	default:
		return NewError("LINK_TYPE_INVALID", "payment link type must be invoice, order, or subscription")
	}
	return nil
}

// LinkKey is the uniqueness tuple.
func (l PaymentLink) LinkKey() string {
	return l.PaymentID + "|" + l.LinkedType + "|" + l.LinkedID
}
