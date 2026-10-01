package query_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
)

var dashAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

func dashPosting(id string, at time.Time, legs int64) entity.PostingData {
	entries := []entity.Entry{
		{ID: valueobject.EntryID("e-" + id + "-d"), PostingID: valueobject.PostingID(id), AccountID: "a-src", Side: valueobject.DirectionDebit, AmountMinor: legs, AssetCode: "USD", AccountSeq: 1},
		{ID: valueobject.EntryID("e-" + id + "-c"), PostingID: valueobject.PostingID(id), AccountID: "a-dst", Side: valueobject.DirectionCredit, AmountMinor: legs, AssetCode: "USD", AccountSeq: 1},
	}
	return entity.PostingData{
		ID: valueobject.PostingID("p-" + id), TenantID: "t-1", LedgerID: "l-1", Operation: "transfer",
		Entries: entries, EffectiveAt: at, RecordedAt: at,
	}
}

func TestDashboardVolume(t *testing.T) {
	t.Parallel()

	t.Run("day buckets sum legs", func(t *testing.T) {
		postings := mockapplication.NewMockPostingQuery(t)
		postings.EXPECT().
			Search(mock.Anything, mock.Anything).
			Return(port.PostingSearchPage{
				Postings: []entity.PostingData{
					dashPosting("1", dashAt, 5000),
					dashPosting("2", dashAt.Add(24*time.Hour), 3000),
				},
				NextCursor: "",
			}, nil).
			Once()

		transfers := mockcommand.NewMockTransferStore(t)

		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings:  postings,
			Transfers: transfers,
		})
		buckets, err := svc.AggregateVolume(context.Background(), "t-1", dashAt.Add(-time.Hour), dashAt.Add(48*time.Hour), "day")
		require.NoError(t, err)
		require.Len(t, buckets, 2)
		assert.Equal(t, "2026-09-15", buckets[0].Bucket)
		assert.Equal(t, int64(1), buckets[0].Count)
		assert.Equal(t, int64(10000), buckets[0].TotalMinor)
		assert.Equal(t, "2026-09-16", buckets[1].Bucket)
	})

	t.Run("week buckets group seven days", func(t *testing.T) {
		postings := mockapplication.NewMockPostingQuery(t)
		postings.EXPECT().
			Search(mock.Anything, mock.Anything).
			Return(port.PostingSearchPage{
				Postings: []entity.PostingData{
					dashPosting("1", dashAt, 5000),
					dashPosting("2", dashAt.Add(24*time.Hour), 3000),
				},
				NextCursor: "",
			}, nil).
			Once()

		transfers := mockcommand.NewMockTransferStore(t)

		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings:  postings,
			Transfers: transfers,
		})
		buckets, err := svc.AggregateVolume(context.Background(), "t-1", dashAt.Add(-time.Hour), dashAt.Add(48*time.Hour), "week")
		require.NoError(t, err)
		require.Len(t, buckets, 1)
		assert.Equal(t, int64(2), buckets[0].Count)
		assert.Equal(t, int64(16000), buckets[0].TotalMinor)
	})

	t.Run("bad granularity rejected", func(t *testing.T) {
		postings := mockapplication.NewMockPostingQuery(t)
		transfers := mockcommand.NewMockTransferStore(t)

		svc := query.NewDashboardService(query.DashboardServiceParams{Postings: postings, Transfers: transfers})
		_, err := svc.AggregateVolume(context.Background(), "t-1", dashAt, dashAt, "fortnight")
		assert.Equal(t, entity.NewError("DASHBOARD_GRANULARITY_INVALID", "granularity must be day or week"), err)
	})

	t.Run("unbounded scan fails loudly", func(t *testing.T) {
		postings := mockapplication.NewMockPostingQuery(t)
		for i := 0; i < 20; i++ {
			cursor := ""
			if i > 0 {
				cursor = fmt.Sprintf("c-%d", i)
			}
			next := fmt.Sprintf("c-%d", i+1)
			postings.EXPECT().
				Search(mock.Anything, port.PostingFilter{
					TenantID: valueobject.TenantID("t-1"),
					Cursor:   cursor,
					Limit:    500,
				}).
				Return(port.PostingSearchPage{Postings: nil, NextCursor: next}, nil).
				Once()
		}

		transfers := mockcommand.NewMockTransferStore(t)

		svc := query.NewDashboardService(query.DashboardServiceParams{Postings: postings, Transfers: transfers})
		_, err := svc.AggregateVolume(context.Background(), "t-1", dashAt.Add(-time.Hour), dashAt.Add(time.Hour), "day")
		assert.Equal(t, entity.NewError("DASHBOARD_TOO_LARGE", "aggregation exceeds the bounded scan"), err)
	})

	t.Run("transfer status counts tally", func(t *testing.T) {
		postings := mockapplication.NewMockPostingQuery(t)
		transfers := mockcommand.NewMockTransferStore(t)
		transfers.EXPECT().
			ListTransfers(mock.Anything, mock.Anything).
			Return([]command.TransferRecord{
				{ID: "x-1", Status: command.TransferCompleted},
				{ID: "x-2", Status: command.TransferCompleted},
				{ID: "x-3", Status: command.TransferFailed},
			}, "", nil).
			Once()

		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings:  postings,
			Transfers: transfers,
		})
		counts, err := svc.TransferStatusCounts(context.Background(), "t-1")
		require.NoError(t, err)
		assert.Equal(t, int64(3), counts.Total)
		assert.Equal(t, int64(2), counts.ByStatus[command.TransferCompleted])
		assert.Equal(t, int64(1), counts.ByStatus[command.TransferFailed])
	})
}

func TestReconReports(t *testing.T) {
	t.Parallel()

	t.Run("summary tallies by status", func(t *testing.T) {
		store := mockcommand.NewMockReconStore(t)
		store.EXPECT().
			FindRun(mock.Anything, valueobject.TenantID("t-1"), "run-1").
			Return(command.ReconRunRecord{ID: "run-1", Status: command.ReconRunCompleted}, nil).
			Once()
		store.EXPECT().
			ListBreaksByRun(mock.Anything, valueobject.TenantID("t-1"), "run-1").
			Return([]command.BreakRecord{
				{Break: entity.ReconciliationBreak{BreakID: "b-1", RunID: "run-1"}, Status: valueobject.BreakResolved},
				{Break: entity.ReconciliationBreak{BreakID: "b-2", RunID: "run-1"}, Status: valueobject.BreakOpen},
			}, nil).
			Twice()

		svc := query.NewReconReportService(query.ReconReportServiceParams{Runs: store})
		summary, err := svc.SummarizeRun(context.Background(), "t-1", "run-1")
		require.NoError(t, err)
		assert.Equal(t, 2, summary.Total)
		assert.Equal(t, 1, summary.ByStatus["RESOLVED"])
		assert.Equal(t, 1, summary.ByStatus["OPEN"])

		filtered, err := svc.BreaksByStatus(context.Background(), "t-1", "run-1", "OPEN", 10)
		require.NoError(t, err)
		require.Len(t, filtered, 1)
		assert.Equal(t, "b-2", filtered[0].Break.BreakID)
	})
}

func TestRegulatorySupportedTypes(t *testing.T) {
	t.Parallel()

	t.Run("four regulatory types supported", func(t *testing.T) {
		svc := query.NewRegulatoryQueryService(query.RegulatoryQueryServiceParams{})
		assert.Len(t, svc.SupportedTypes(), 4)
	})
}
