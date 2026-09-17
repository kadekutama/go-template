package postgres

// Lag metric contract for E15-T02 alerts (E07-T07).

// LagMetricName is the gauge scraped for replication lag.
const LagMetricName = "replication_lag_seconds"

// LagAlertThresholdSeconds is the default alert threshold shape reviewed
// against E15-T02. Deployments override it per environment.
const LagAlertThresholdSeconds = 30.0

// LagAlertRule describes the alert shape for the observability stack.
type LagAlertRule struct {
	Metric    string
	Threshold float64
	Severity  string
}

// DefaultLagAlertRule returns the canonical alert shape.
func DefaultLagAlertRule() LagAlertRule {
	return LagAlertRule{
		Metric:    LagMetricName,
		Threshold: LagAlertThresholdSeconds,
		Severity:  "warning",
	}
}

// StaticLagSource is a deterministic LagSource for tests and local wiring.
type StaticLagSource struct {
	Lag float64
	Err error
}

// Sample returns the canned lag.
func (s StaticLagSource) Sample() (float64, error) {
	if s.Err != nil {
		return 0, s.Err
	}

	return s.Lag, nil
}
