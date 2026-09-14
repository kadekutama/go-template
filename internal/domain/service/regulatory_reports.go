package service

import (
	"github.com/kadekutama/go-template/internal/domain/entity"
)

// ReportType is the regulatory report definition.
type ReportType string

// Regulatory report types.
const (
	ReportCallReport ReportType = "CALL_REPORT"
	ReportForm1099   ReportType = "FORM_1099"
	ReportFATCA      ReportType = "FATCA"
	ReportCRS        ReportType = "CRS"
)

// Regulatory report field keys.
const (
	fieldTenantID    = "tenant_id"
	fieldRuleVersion = "rule_version"
)

// ReportDefinitionVersion is the current versioned field-definition set.
const ReportDefinitionVersion = "v1"

// ReportDefinition is the versioned field list plus aggregation rule.
type ReportDefinition struct {
	Type            ReportType
	Version         string
	RequiredFields  []string
	AggregationRule string
}

// ReportDefinitionFor returns the versioned definition per report type.
func ReportDefinitionFor(t ReportType) (ReportDefinition, error) {
	switch t {
	case ReportCallReport:
		return ReportDefinition{Type: t, Version: ReportDefinitionVersion, RequiredFields: []string{fieldTenantID, "period_start", "period_end", "total_assets_minor", "total_liabilities_minor", fieldRuleVersion}, AggregationRule: "sum entries by asset per period; platform books only"}, nil
	case ReportForm1099:
		return ReportDefinition{Type: t, Version: ReportDefinitionVersion, RequiredFields: []string{fieldTenantID, "tax_year", "payer_tin_hash", "payee_count", "gross_amount_minor", fieldRuleVersion}, AggregationRule: "aggregate payouts per payee per tax year"}, nil
	case ReportFATCA:
		return ReportDefinition{Type: t, Version: ReportDefinitionVersion, RequiredFields: []string{fieldTenantID, "reporting_period", "account_holder_hash", "account_balance_minor", "jurisdiction", fieldRuleVersion}, AggregationRule: "snapshot balances per holder per period with jurisdiction"}, nil
	case ReportCRS:
		return ReportDefinition{Type: t, Version: ReportDefinitionVersion, RequiredFields: []string{fieldTenantID, "reporting_period", "account_holder_hash", "account_balance_minor", "residence_jurisdiction", fieldRuleVersion}, AggregationRule: "snapshot balances per holder per period with residence"}, nil
	default:
		return ReportDefinition{}, entity.NewError("REPORT_TYPE_UNKNOWN", "regulatory report type is unknown")
	}
}

// ReportFields returns the versioned required field list per report type.
func ReportFields(t ReportType) ([]string, error) {
	def, err := ReportDefinitionFor(t)
	if err != nil {
		return nil, err
	}
	return append([]string(nil), def.RequiredFields...), nil
}

// ValidateReport rejects missing required fields and unknown types.
func ValidateReport(t ReportType, fields map[string]string) error {
	required, err := ReportFields(t)
	if err != nil {
		return err
	}
	if len(required) == 0 {
		return entity.NewError("REPORT_FIELDS_REQUIRED", "regulatory report requires fields")
	}
	for _, name := range required {
		if fields[name] == "" {
			return entity.Errorf("REPORT_FIELD_MISSING", "report field %s is required", name)
		}
	}
	return nil
}
