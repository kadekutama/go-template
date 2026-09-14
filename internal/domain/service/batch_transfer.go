package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// BatchItem is one independent transfer inside a bulk batch.
type BatchItem struct {
	Key     string
	Request TransferRequest
}

// BatchItemResult is the per-item outcome. Sibling failure never rolls back
// siblings; each item carries its own code.
type BatchItemResult struct {
	Key     string
	OK      bool
	Code    string
	Message string
}

// BatchResult aggregates per-item outcomes. Status is COMPLETED (all ok),
// PARTIAL (mixed), or FAILED (none ok).
type BatchResult struct {
	BatchID string
	Status  string
	Items   []BatchItemResult
}

// Batch statuses.
const (
	BatchCompleted = "COMPLETED"
	BatchPartial   = "PARTIAL"
	BatchFailed    = "FAILED"
)

// BatchItemKey scopes an item key under the batch idempotency root.
func BatchItemKey(batchID, itemKey string) string {
	return batchID + ":" + itemKey
}

// EvaluateBatch runs fn per item independently and aggregates the result.
// fn is the per-item validator/executor supplied by the caller (usually
// ValidateImmediate against per-source snapshots); the batch never shares
// failure state between items.
func EvaluateBatch(batchID string, items []BatchItem, fn func(BatchItem) error) BatchResult {
	if batchID == "" {
		return BatchResult{BatchID: batchID, Status: BatchFailed, Items: []BatchItemResult{
			{Key: "", OK: false, Code: "BATCH_ID_REQUIRED", Message: "batch id is required"},
		}}
	}
	if len(items) == 0 {
		return BatchResult{BatchID: batchID, Status: BatchFailed, Items: nil}
	}
	res := BatchResult{BatchID: batchID, Items: make([]BatchItemResult, 0, len(items))}
	okCount := 0
	for _, it := range items {
		if it.Key == "" {
			res.Items = append(res.Items, BatchItemResult{Key: "", OK: false, Code: "BATCH_ITEM_REQUIRED", Message: "batch item key is required"})
			continue
		}
		if err := fn(it); err != nil {
			res.Items = append(res.Items, BatchItemResult{Key: it.Key, OK: false, Code: domainCode(err), Message: err.Error()})
			continue
		}
		okCount++
		res.Items = append(res.Items, BatchItemResult{Key: it.Key, OK: true, Code: "OK", Message: "ok"})
	}
	switch {
	case okCount == len(items):
		res.Status = BatchCompleted
	case okCount == 0:
		res.Status = BatchFailed
	default:
		res.Status = BatchPartial
	}
	return res
}

func domainCode(err error) string {
	if e, ok := err.(*entity.Error); ok && e != nil && e.Code != "" {
		return e.Code
	}
	return "BATCH_ITEM_FAILED"
}
