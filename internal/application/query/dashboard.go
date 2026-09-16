package query

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

// dashboardScanPages caps the pages scanned per aggregation: aggregations
// stay bounded and fail loudly instead of scanning forever.
const dashboardScanPages = 20

// VolumeBucket is one day/week bucket with posted counts and sums.
type VolumeBucket struct {
	Bucket     string
	Count      int64
	TotalMinor int64
}

// StatusCounts tallies transfer intents by lifecycle state.
type StatusCounts struct {
	Total    int64
	ByStatus map[string]int64
}

// DashboardServiceParams encapsulates dependencies for DashboardService.
type DashboardServiceParams struct {
	Postings  port.PostingQuery
	Transfers TransferReader
}

// DashboardService aggregates bounded operational views: posting volume by
// day/week and transfer status counts. It reads committed facts only; spend
// decisions never use these views.
type DashboardService struct {
	postings  port.PostingQuery
	transfers TransferReader
}

// NewDashboardService creates an encapsulated DashboardService with validated dependencies.
func NewDashboardService(params DashboardServiceParams) *DashboardService {
	return &DashboardService{
		postings:  params.Postings,
		transfers: params.Transfers,
	}
}

// AggregateVolume groups committed postings by day ("day") or ISO week
// ("week") over an inclusive range, summing absolute minor units. Bounded:
// at most dashboardScanPages pages are scanned.
func (s *DashboardService) AggregateVolume(ctx context.Context, tenant valueobject.TenantID, from, to time.Time, granularity string) ([]VolumeBucket, error) {
	if granularity != "day" && granularity != "week" {
		return nil, entity.NewError("DASHBOARD_GRANULARITY_INVALID", "granularity must be day or week")
	}
	if to.Before(from) {
		return nil, entity.NewError("DASHBOARD_RANGE_INVALID", "to must not precede from")
	}
	buckets := map[string]*VolumeBucket{}
	if err := s.scanPostings(ctx, tenant, from, to, granularity, buckets); err != nil {
		return nil, err
	}
	out := make([]VolumeBucket, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, *bucket)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bucket < out[j].Bucket })
	return out, nil
}

// scanPostings pages committed postings into day/week buckets, stopping at
// the page cap, an empty cursor, or a non-progressing cursor.
func (s *DashboardService) scanPostings(ctx context.Context, tenant valueobject.TenantID, from, to time.Time, granularity string, buckets map[string]*VolumeBucket) error {
	cursor := ""
	for pages := 0; ; pages++ {
		if pages >= dashboardScanPages {
			return entity.NewError("DASHBOARD_TOO_LARGE", "aggregation exceeds the bounded scan")
		}
		page, err := s.postings.Search(ctx, port.PostingFilter{TenantID: tenant, Cursor: cursor, Limit: 500})
		if err != nil {
			return err
		}
		for _, posting := range page.Postings {
			accumulateBucket(buckets, posting, from, to, granularity)
		}
		if page.NextCursor == "" || page.NextCursor == cursor {
			return nil
		}
		cursor = page.NextCursor
	}
}

// accumulateBucket folds one posting into its day/week bucket when inside
// the inclusive range.
func accumulateBucket(buckets map[string]*VolumeBucket, posting entity.PostingData, from, to time.Time, granularity string) {
	if posting.RecordedAt.Before(from) || posting.RecordedAt.After(to) {
		return
	}
	key := posting.RecordedAt.UTC().Format("2006-01-02")
	if granularity == "week" {
		year, week := posting.RecordedAt.UTC().ISOWeek()
		key = fmt.Sprintf("%04d-W%02d", year, week)
	}
	bucket, ok := buckets[key]
	if !ok {
		bucket = &VolumeBucket{Bucket: key}
		buckets[key] = bucket
	}
	bucket.Count++
	bucket.TotalMinor += postingTotal(posting)
}

// TransferStatusCounts tallies transfer intents by lifecycle state for one
// tenant. Bounded by the store page.
func (s *DashboardService) TransferStatusCounts(ctx context.Context, tenant valueobject.TenantID) (StatusCounts, error) {
	records, _, err := s.transfers.ListTransfers(ctx, port.TransferListFilter{TenantID: tenant, Limit: 500})
	if err != nil {
		return StatusCounts{}, err
	}
	counts := StatusCounts{ByStatus: map[string]int64{}}
	for _, record := range records {
		counts.ByStatus[record.Status]++
		counts.Total++
	}
	return counts, nil
}

// postingTotal sums absolute entry legs for one posting.
func postingTotal(posting entity.PostingData) int64 {
	var total int64
	for _, entry := range posting.Entries {
		total += entry.AmountMinor
	}
	return total
}
