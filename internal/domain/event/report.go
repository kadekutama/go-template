package event

import (
	"time"
)

// ReportGeneratedPayload notifies that async report rendering finished.
// Unlike §§3.1–3.10 events it originates from the reporting application
// service (read-model), not from an aggregate, but it travels the same
// webhook pipeline.
type ReportGeneratedPayload struct {
	ReportID    string `json:"report_id"`
	TenantID    string `json:"tenant_id"`
	Template    string `json:"template"`
	Format      string `json:"format"`
	DownloadURL string `json:"download_url"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
}

// NewReportGenerated builds report.generated.v1.
func NewReportGenerated(eventID, aggregateID string, occurredAt time.Time, version, seq int64, payload ReportGeneratedPayload, meta EventMetadata) (TypedEvent[ReportGeneratedPayload], error) {
	if err := requireIDs(map[string]string{"report_id": payload.ReportID, fieldTenantID: payload.TenantID, "template": payload.Template}); err != nil {
		return TypedEvent[ReportGeneratedPayload]{}, err
	}
	return newTyped("report.generated.v1", eventID, aggregateID, "Report", occurredAt, version, seq, payload, meta)
}
