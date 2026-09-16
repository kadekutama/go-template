package query_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

var dashAt = time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)

type dashPostings struct {
	pages map[string]port.PostingSearchPage
}

func (s *dashPostings) Search(_ context.Context, filter port.PostingFilter) (port.PostingSearchPage, error) {
	return s.pages[filter.Cursor], nil
}

type dashTransfers struct {
	records []command.TransferRecord
}

func (s *dashTransfers) CreateTransfer(_ context.Context, _ command.TransferRecord) error {
	return nil
}

func (s *dashTransfers) FindTransfer(_ context.Context, _ valueobject.TenantID, _ string) (command.TransferRecord, error) {
	return command.TransferRecord{}, nil
}

func (s *dashTransfers) UpdateTransfer(_ context.Context, _ command.TransferRecord) error {
	return nil
}

func (s *dashTransfers) ListTransfers(_ context.Context, _ command.TransferListFilter) ([]command.TransferRecord, string, error) {
	return s.records, "", nil
}

func (s *dashTransfers) CreateBatch(_ context.Context, _ command.BatchRecord, _ []command.BatchItem) error {
	return nil
}

func (s *dashTransfers) FindBatch(_ context.Context, _ valueobject.TenantID, _ string) (command.BatchRecord, error) {
	return command.BatchRecord{}, nil
}

func (s *dashTransfers) UpdateBatchState(_ context.Context, _ valueobject.TenantID, _, _ string) error {
	return nil
}

func (s *dashTransfers) UpdateBatchItem(_ context.Context, _ valueobject.TenantID, _ string, _ int, _, _ string) error {
	return nil
}

func (s *dashTransfers) ListBatchItems(_ context.Context, _ valueobject.TenantID, _ string) ([]command.BatchItem, error) {
	return nil, nil
}

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
		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings: &dashPostings{pages: map[string]port.PostingSearchPage{
				"": {
					Postings: []entity.PostingData{
						dashPosting("1", dashAt, 5000),
						dashPosting("2", dashAt.Add(24*time.Hour), 3000),
					},
					NextCursor: "",
				},
			}},
			Transfers: &dashTransfers{},
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
		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings: &dashPostings{pages: map[string]port.PostingSearchPage{
				"": {
					Postings: []entity.PostingData{
						dashPosting("1", dashAt, 5000),
						dashPosting("2", dashAt.Add(24*time.Hour), 3000),
					},
					NextCursor: "",
				},
			}},
			Transfers: &dashTransfers{},
		})
		buckets, err := svc.AggregateVolume(context.Background(), "t-1", dashAt.Add(-time.Hour), dashAt.Add(48*time.Hour), "week")
		require.NoError(t, err)
		require.Len(t, buckets, 1)
		assert.Equal(t, int64(2), buckets[0].Count)
		assert.Equal(t, int64(16000), buckets[0].TotalMinor)
	})

	t.Run("bad granularity rejected", func(t *testing.T) {
		svc := query.NewDashboardService(query.DashboardServiceParams{Postings: &dashPostings{pages: map[string]port.PostingSearchPage{}}, Transfers: &dashTransfers{}})
		_, err := svc.AggregateVolume(context.Background(), "t-1", dashAt, dashAt, "fortnight")
		assert.Equal(t, entity.NewError("DASHBOARD_GRANULARITY_INVALID", "granularity must be day or week"), err)
	})

	t.Run("unbounded scan fails loudly", func(t *testing.T) {
		pages := map[string]port.PostingSearchPage{}
		for i := 0; i < 25; i++ {
			next := ""
			if i < 24 {
				next = fmt.Sprintf("c-%d", i+1)
			}
			cursor := ""
			if i > 0 {
				cursor = fmt.Sprintf("c-%d", i)
			}
			pages[cursor] = port.PostingSearchPage{Postings: nil, NextCursor: next}
		}
		svc := query.NewDashboardService(query.DashboardServiceParams{Postings: &dashPostings{pages: pages}, Transfers: &dashTransfers{}})
		_, err := svc.AggregateVolume(context.Background(), "t-1", dashAt.Add(-time.Hour), dashAt.Add(time.Hour), "day")
		assert.Equal(t, entity.NewError("DASHBOARD_TOO_LARGE", "aggregation exceeds the bounded scan"), err)
	})

	t.Run("transfer status counts tally", func(t *testing.T) {
		svc := query.NewDashboardService(query.DashboardServiceParams{
			Postings: &dashPostings{pages: map[string]port.PostingSearchPage{}},
			Transfers: &dashTransfers{records: []command.TransferRecord{
				{ID: "x-1", Status: command.TransferCompleted},
				{ID: "x-2", Status: command.TransferCompleted},
				{ID: "x-3", Status: command.TransferFailed},
			}},
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
		store := &reportReconStore{
			run: command.ReconRunRecord{ID: "run-1", Status: command.ReconRunCompleted},
			breaks: []command.BreakRecord{
				{Break: entity.ReconciliationBreak{BreakID: "b-1", RunID: "run-1"}, Status: valueobject.BreakResolved},
				{Break: entity.ReconciliationBreak{BreakID: "b-2", RunID: "run-1"}, Status: valueobject.BreakOpen},
				{Break: entity.ReconciliationBreak{BreakID: "b-3", RunID: "run-9"}, Status: valueobject.BreakOpen},
			},
		}
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

type reportReconStore struct {
	run    command.ReconRunRecord
	breaks []command.BreakRecord
}

func (s *reportReconStore) CreateRun(_ context.Context, _ command.ReconRunRecord) error {
	return nil
}

func (s *reportReconStore) FindRun(_ context.Context, _ valueobject.TenantID, _ string) (command.ReconRunRecord, error) {
	return s.run, nil
}

func (s *reportReconStore) UpdateRun(_ context.Context, _ command.ReconRunRecord) error {
	return nil
}

func (s *reportReconStore) CreateBreaks(_ context.Context, _ []command.BreakRecord) error {
	return nil
}

func (s *reportReconStore) FindBreak(_ context.Context, _ valueobject.TenantID, _ string) (command.BreakRecord, error) {
	return command.BreakRecord{}, nil
}

func (s *reportReconStore) UpdateBreak(_ context.Context, _ command.BreakRecord) error {
	return nil
}

func (s *reportReconStore) CountOpenBreaks(_ context.Context, _ valueobject.TenantID, _ valueobject.LedgerID) (int, error) {
	return 0, nil
}

func (s *reportReconStore) ListRuns(_ context.Context, _ valueobject.TenantID, _ int) ([]command.ReconRunRecord, error) {
	return nil, nil
}

func (s *reportReconStore) ListBreaks(_ context.Context, _ valueobject.TenantID, _ string, _ int) ([]command.BreakRecord, error) {
	return nil, nil
}

func (s *reportReconStore) ListBreaksByRun(_ context.Context, _ valueobject.TenantID, runID string) ([]command.BreakRecord, error) {
	var out []command.BreakRecord
	for _, record := range s.breaks {
		if record.Break.RunID == runID {
			out = append(out, record)
		}
	}
	return out, nil
}

func TestRegulatorySupportedTypes(t *testing.T) {
	t.Parallel()

	t.Run("four regulatory types supported", func(t *testing.T) {
		svc := query.NewRegulatoryQueryService(query.RegulatoryQueryServiceParams{})
		assert.Len(t, svc.SupportedTypes(), 4)
	})
}
