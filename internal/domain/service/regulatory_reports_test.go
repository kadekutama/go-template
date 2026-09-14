package service_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/service"
)

func TestReportFields(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		t              service.ReportType
		expectedResult []string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "call report fields",
			t:              service.ReportCallReport,
			expectedResult: []string{"tenant_id", "period_start", "period_end", "total_assets_minor", "total_liabilities_minor", "rule_version"},
			expectedError:  nil,
		},
		{
			name:           "form 1099 fields",
			t:              service.ReportForm1099,
			expectedResult: []string{"tenant_id", "tax_year", "payer_tin_hash", "payee_count", "gross_amount_minor", "rule_version"},
			expectedError:  nil,
		},
		{
			name:           "fatca fields",
			t:              service.ReportFATCA,
			expectedResult: []string{"tenant_id", "reporting_period", "account_holder_hash", "account_balance_minor", "jurisdiction", "rule_version"},
			expectedError:  nil,
		},
		{
			name:           "crs fields",
			t:              service.ReportCRS,
			expectedResult: []string{"tenant_id", "reporting_period", "account_holder_hash", "account_balance_minor", "residence_jurisdiction", "rule_version"},
			expectedError:  nil,
		},
		{
			name:           "unknown type",
			t:              service.ReportType("W2"),
			expectedResult: nil,
			expectedError:  entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
		{
			name:           "empty type",
			t:              service.ReportType(""),
			expectedResult: nil,
			expectedError:  entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ReportFields(tc.t)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestReportDefinitionFor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		t              service.ReportType
		expectedResult service.ReportDefinition
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "call report definition",
			t:    service.ReportCallReport,
			expectedResult: service.ReportDefinition{
				Type:            service.ReportCallReport,
				Version:         service.ReportDefinitionVersion,
				RequiredFields:  []string{"tenant_id", "period_start", "period_end", "total_assets_minor", "total_liabilities_minor", "rule_version"},
				AggregationRule: "sum entries by asset per period; platform books only",
			},
			expectedError: nil,
		},
		{
			name: "form 1099 definition",
			t:    service.ReportForm1099,
			expectedResult: service.ReportDefinition{
				Type:            service.ReportForm1099,
				Version:         service.ReportDefinitionVersion,
				RequiredFields:  []string{"tenant_id", "tax_year", "payer_tin_hash", "payee_count", "gross_amount_minor", "rule_version"},
				AggregationRule: "aggregate payouts per payee per tax year",
			},
			expectedError: nil,
		},
		{
			name: "fatca definition",
			t:    service.ReportFATCA,
			expectedResult: service.ReportDefinition{
				Type:            service.ReportFATCA,
				Version:         service.ReportDefinitionVersion,
				RequiredFields:  []string{"tenant_id", "reporting_period", "account_holder_hash", "account_balance_minor", "jurisdiction", "rule_version"},
				AggregationRule: "snapshot balances per holder per period with jurisdiction",
			},
			expectedError: nil,
		},
		{
			name: "crs definition",
			t:    service.ReportCRS,
			expectedResult: service.ReportDefinition{
				Type:            service.ReportCRS,
				Version:         service.ReportDefinitionVersion,
				RequiredFields:  []string{"tenant_id", "reporting_period", "account_holder_hash", "account_balance_minor", "residence_jurisdiction", "rule_version"},
				AggregationRule: "snapshot balances per holder per period with residence",
			},
			expectedError: nil,
		},
		{
			name:           "unknown type",
			t:              service.ReportType("W2"),
			expectedResult: service.ReportDefinition{},
			expectedError:  entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
		{
			name:           "empty type",
			t:              service.ReportType(""),
			expectedResult: service.ReportDefinition{},
			expectedError:  entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := service.ReportDefinitionFor(tc.t)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestValidateRegulatoryReport(t *testing.T) {
	t.Parallel()

	baseCallFields := map[string]string{
		"tenant_id":               "t-1",
		"period_start":            "2026-09-01",
		"period_end":              "2026-09-30",
		"total_assets_minor":      "100000",
		"total_liabilities_minor": "90000",
		"rule_version":            "v1",
	}

	type testCase struct {
		name          string
		t             service.ReportType
		fields        map[string]string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid call report",
			t:             service.ReportCallReport,
			fields:        baseCallFields,
			expectedError: nil,
		},
		{
			name: "valid form 1099",
			t:    service.ReportForm1099,
			fields: map[string]string{
				"tenant_id":          "t-1",
				"tax_year":           "2026",
				"payer_tin_hash":     "hash-123",
				"payee_count":        "42",
				"gross_amount_minor": "1000000",
				"rule_version":       "v1",
			},
			expectedError: nil,
		},
		{
			name: "valid fatca report",
			t:    service.ReportFATCA,
			fields: map[string]string{
				"tenant_id":             "t-1",
				"reporting_period":      "2026-Q3",
				"account_holder_hash":   "holder-123",
				"account_balance_minor": "50000",
				"jurisdiction":          "US",
				"rule_version":          "v1",
			},
			expectedError: nil,
		},
		{
			name: "valid crs report",
			t:    service.ReportCRS,
			fields: map[string]string{
				"tenant_id":              "t-1",
				"reporting_period":       "2026-Q3",
				"account_holder_hash":    "holder-123",
				"account_balance_minor":  "50000",
				"residence_jurisdiction": "GB",
				"rule_version":           "v1",
			},
			expectedError: nil,
		},
		{
			name: "missing field rejected",
			t:    service.ReportCallReport,
			fields: func() map[string]string {
				m := make(map[string]string, len(baseCallFields))
				for k, v := range baseCallFields {
					m[k] = v
				}
				delete(m, "total_assets_minor")
				return m
			}(),
			expectedError: entity.Errorf("REPORT_FIELD_MISSING", "report field %s is required", "total_assets_minor"),
		},
		{
			name: "blank field rejected",
			t:    service.ReportCallReport,
			fields: func() map[string]string {
				m := make(map[string]string, len(baseCallFields))
				for k, v := range baseCallFields {
					m[k] = v
				}
				m["period_start"] = ""
				return m
			}(),
			expectedError: entity.Errorf("REPORT_FIELD_MISSING", "report field %s is required", "period_start"),
		},
		{
			name:          "unknown type rejected",
			t:             service.ReportType("W2"),
			fields:        baseCallFields,
			expectedError: entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
		{
			name:          "empty type rejected",
			t:             service.ReportType(""),
			fields:        baseCallFields,
			expectedError: entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := service.ValidateReport(tc.t, tc.fields)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
