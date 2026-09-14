package entity

// ProviderObjectLink is the immutable provider-object mapping: one provider
// object (or webhook event) maps to exactly one internal result. Duplicate
// deliveries return the existing state without another posting.
type ProviderObjectLink struct {
	ProviderObjectID string
	EventID          string
	PaymentID        string
	PostingID        string
}

// Validate checks the mapping identity.
func (l ProviderObjectLink) Validate() error {
	if l.ProviderObjectID == "" {
		return NewError("PROVIDER_OBJECT_REQUIRED", "provider object id is required")
	}
	if l.PaymentID == "" {
		return NewError("PAYMENT_ID_REQUIRED", "provider mapping requires a payment id")
	}
	return nil
}

// DeliveryKey scopes idempotency to the provider object + event.
func (l ProviderObjectLink) DeliveryKey() string {
	return l.ProviderObjectID + "|" + l.EventID
}
