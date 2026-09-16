package dto

import (
	"github.com/kadekutama/go-template/internal/application/service"
)

// UsageLineDTO is one tenant/day/kind usage line on the edge.
type UsageLineDTO struct {
	Tenant   string `json:"tenant"`
	Day      string `json:"day"`
	Kind     string `json:"kind"`
	Quantity int64  `json:"quantity"`
}

// BillingExportDTO is one invoice-ready export with its lines and total.
type BillingExportDTO struct {
	Lines []UsageLineDTO `json:"lines"`
	Total int64          `json:"total"`
}

// ToBillingExport maps stored daily lines to the edge with a summed total.
func ToBillingExport(lines []service.DailyUsage) BillingExportDTO {
	dto := BillingExportDTO{Lines: make([]UsageLineDTO, 0, len(lines))}
	for _, line := range lines {
		dto.Lines = append(dto.Lines, UsageLineDTO{
			Tenant: line.TenantID, Day: line.Day, Kind: string(line.Kind), Quantity: line.Quantity,
		})
		dto.Total += line.Quantity
	}
	return dto
}
