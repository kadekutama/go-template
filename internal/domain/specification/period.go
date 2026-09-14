package specification

import (
	"context"

	"github.com/kadekutama/go-template/internal/domain/entity"
)

// PeriodOpen passes only for OPEN periods: no posting into CLOSED periods.
func PeriodOpen() Specification[entity.PeriodData] {
	return NewFuncSpec[entity.PeriodData]("PERIOD_CLOSED", "period is closed",
		func(_ context.Context, p entity.PeriodData) bool { return p.Status == entity.PeriodOpen })
}
