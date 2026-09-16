package query

import (
	"context"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// TransferReader defines the consumer-owned read operations required for transfer and batch queries.
type TransferReader interface {
	FindTransfer(ctx context.Context, tenant valueobject.TenantID, id string) (port.TransferRecord, error)
	ListTransfers(ctx context.Context, filter port.TransferListFilter) ([]port.TransferRecord, string, error)
	FindBatch(ctx context.Context, tenant valueobject.TenantID, id string) (port.BatchRecord, error)
	ListBatchItems(ctx context.Context, tenant valueobject.TenantID, batchID string) ([]port.BatchItem, error)
}

// TransferQueryServiceParams encapsulates dependencies for TransferQueryService.
type TransferQueryServiceParams struct {
	Transfers TransferReader
}

// TransferQueryService serves transfer and batch reads for the edge by
// querying the transfer store directly. Reads are strong (Get) or
// point-in-time (List/Status).
type TransferQueryService struct {
	transfers TransferReader
}

// NewTransferQueryService creates an encapsulated TransferQueryService with validated dependencies.
func NewTransferQueryService(params TransferQueryServiceParams) *TransferQueryService {
	return &TransferQueryService{
		transfers: params.Transfers,
	}
}

var _ port.TransferQueryUseCases = (*TransferQueryService)(nil)

// GetTransfer returns one transfer. Strong read.
func (s *TransferQueryService) GetTransfer(ctx context.Context, query port.TransferQuery) (port.TransferView, error) {
	record, err := s.transfers.FindTransfer(ctx, query.TenantID, query.TransferID)
	if err != nil {
		return port.TransferView{}, err
	}
	return port.TransferView{TransferID: record.ID, Status: record.Status, PostingID: record.PostingID}, nil
}

// ListTransfers pages transfers by filter. Point-in-time page.
func (s *TransferQueryService) ListTransfers(ctx context.Context, filter port.TransferFilter) (port.TransferPage, error) {
	records, next, err := s.transfers.ListTransfers(ctx, port.TransferListFilter(filter))
	if err != nil {
		return port.TransferPage{}, err
	}
	views := make([]port.TransferView, 0, len(records))
	for _, record := range records {
		views = append(views, port.TransferView{TransferID: record.ID, Status: record.Status, PostingID: record.PostingID})
	}
	return port.TransferPage{Transfers: views, NextCursor: next}, nil
}

// GetBatchStatus reports batch + item states. Point-in-time read.
func (s *TransferQueryService) GetBatchStatus(ctx context.Context, query port.BatchStatusQuery) (port.BatchStatusResult, error) {
	batch, err := s.transfers.FindBatch(ctx, query.TenantID, query.BatchID)
	if err != nil {
		return port.BatchStatusResult{}, err
	}
	items, err := s.transfers.ListBatchItems(ctx, query.TenantID, query.BatchID)
	if err != nil {
		return port.BatchStatusResult{}, err
	}
	statuses := make([]port.BatchItemStatus, 0, len(items))
	succeeded := 0
	failed := 0
	for _, item := range items {
		switch item.Status {
		case port.TransferCompleted:
			succeeded++
		case port.TransferFailed:
			failed++
		}
		statuses = append(statuses, port.BatchItemStatus{
			Index: item.Index, TransferID: item.TransferID, Status: item.Status, ErrorCode: item.ErrorCode,
		})
	}
	return port.BatchStatusResult{
		BatchID: batch.ID, State: batch.State, Succeeded: succeeded, Failed: failed, Items: statuses,
	}, nil
}
