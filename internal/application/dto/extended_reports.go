package dto

import (
	"time"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/query"
)

// RunSummaryDTO is one reconciliation run summary on the edge.
type RunSummaryDTO struct {
	RunID    string         `json:"run_id"`
	Status   string         `json:"status"`
	Total    int            `json:"total"`
	ByStatus map[string]int `json:"by_status"`
}

// ToRunSummaryDTO maps one run summary.
func ToRunSummaryDTO(summary query.RunSummary) RunSummaryDTO {
	return RunSummaryDTO{
		RunID: summary.RunID, Status: summary.Status, Total: summary.Total, ByStatus: summary.ByStatus,
	}
}

// SubscriptionDTO is one report subscription on the edge.
type SubscriptionDTO struct {
	ID          string    `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Template    string    `json:"template"`
	Cron        string    `json:"cron"`
	Destination string    `json:"destination"`
	LastRun     time.Time `json:"last_run,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ToSubscriptionDTO maps one stored subscription.
func ToSubscriptionDTO(sub command.Subscription) SubscriptionDTO {
	return SubscriptionDTO{
		ID: sub.ID, TenantID: sub.TenantID.String(), Template: sub.Template,
		Cron: sub.Cron, Destination: sub.Destination, LastRun: sub.LastRun, CreatedAt: sub.CreatedAt,
	}
}

// VolumeBucketDTO is one aggregation bucket on the edge.
type VolumeBucketDTO struct {
	Bucket     string `json:"bucket"`
	Count      int64  `json:"count"`
	TotalMinor int64  `json:"total_minor"`
}

// ToVolumeBuckets maps aggregation buckets.
func ToVolumeBuckets(buckets []query.VolumeBucket) []VolumeBucketDTO {
	out := make([]VolumeBucketDTO, 0, len(buckets))
	for _, bucket := range buckets {
		out = append(out, VolumeBucketDTO{Bucket: bucket.Bucket, Count: bucket.Count, TotalMinor: bucket.TotalMinor})
	}
	return out
}
