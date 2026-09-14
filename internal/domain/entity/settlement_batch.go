package entity

// SettlementItemStatus is the per-item outcome inside a provider batch.
type SettlementItemStatus string

// Per-item states.
const (
	SettleItemPending SettlementItemStatus = "PENDING"
	SettleItemSettled SettlementItemStatus = "SETTLED"
	SettleItemFailed  SettlementItemStatus = "FAILED"
)

// SettlementItem is one payment inside a provider settlement batch.
type SettlementItem struct {
	PaymentID   string
	Status      SettlementItemStatus
	AmountMinor int64
}

// SettlementBatch retains the immutable provider object mapping: trace IDs,
// gross/fee/net totals, asset, coverage window, and per-item status. Partial
// batches reconcile each item; duplicates return existing state.
type SettlementBatch struct {
	BatchID         string
	ProviderBatchID string
	ProviderTraceID string
	AssetCode       string
	GrossMinor      int64
	FeeMinor        int64
	NetMinor        int64
	CoverageStart   string
	CoverageEnd     string
	Items           []SettlementItem
}

// Validate checks batch identity and totals (gross == fee + net).
func (b SettlementBatch) Validate() error {
	if b.BatchID == "" || b.ProviderBatchID == "" {
		return NewError("BATCH_ID_REQUIRED", "settlement batch requires batch and provider batch ids")
	}
	if b.ProviderTraceID == "" {
		return NewError("TRACE_ID_REQUIRED", "settlement batch requires a provider trace id")
	}
	if b.AssetCode == "" {
		return NewError("BATCH_ASSET_REQUIRED", "settlement batch requires an asset code")
	}
	if b.GrossMinor < 0 || b.FeeMinor < 0 || b.NetMinor < 0 {
		return NewError("BATCH_TOTAL_INVALID", "settlement batch totals must be non-negative")
	}
	if b.GrossMinor != b.FeeMinor+b.NetMinor {
		return NewError("BATCH_TOTAL_MISMATCH", "settlement batch gross must equal fee plus net")
	}
	if b.CoverageStart == "" || b.CoverageEnd == "" {
		return NewError("BATCH_WINDOW_REQUIRED", "settlement batch requires a coverage window")
	}
	return b.validateItems()
}

func (b SettlementBatch) validateItems() error {
	if len(b.Items) == 0 {
		return NewError("BATCH_ITEMS_REQUIRED", "settlement batch requires at least one item")
	}
	for _, it := range b.Items {
		if it.PaymentID == "" {
			return NewError("BATCH_ITEM_PAYMENT_REQUIRED", "settlement item requires a payment id")
		}
		if it.AmountMinor <= 0 {
			return NewError("INVALID_ENTRY_AMOUNT", "settlement item amount must be positive")
		}
		switch it.Status {
		case SettleItemPending, SettleItemSettled, SettleItemFailed:
		default:
			return NewError("BATCH_ITEM_STATUS_INVALID", "settlement item status is invalid")
		}
	}
	return nil
}
