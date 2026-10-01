package postgres

import (
	"errors"
	"strings"
)

// Lag metric contract for E15-T02 alerts (E07-T07).

// LagAlertRule describes the alert shape for the observability stack.
// All fields must be explicitly configured and valid.
type LagAlertRule struct {
	Metric    string
	Threshold float64
	Severity  string
}

// Validate ensures the alert rule configuration is non-empty and has a positive threshold.
func (r LagAlertRule) Validate() error {
	if strings.TrimSpace(r.Metric) == "" {
		return errors.New("postgres: lag alert metric is required")
	}
	if r.Threshold <= 0 {
		return errors.New("postgres: lag alert threshold must be positive")
	}
	if strings.TrimSpace(r.Severity) == "" {
		return errors.New("postgres: lag alert severity is required")
	}
	return nil
}

// NewLagAlertRule builds and validates a LagAlertRule.
func NewLagAlertRule(metric string, threshold float64, severity string) (LagAlertRule, error) {
	rule := LagAlertRule{
		Metric:    strings.TrimSpace(metric),
		Threshold: threshold,
		Severity:  strings.TrimSpace(severity),
	}
	if err := rule.Validate(); err != nil {
		return LagAlertRule{}, err
	}
	return rule, nil
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
