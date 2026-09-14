package specification_test

import (
	"context"
	"testing"
	"time"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/specification"
)

func TestPeriodOpen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	open := entity.PeriodData{ID: "pd-1", TenantID: testTenant1, LedgerID: testLedger1,
		Start:    time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		Timezone: "UTC", Status: entity.PeriodOpen, Version: 1}
	if res := specification.PeriodOpen().Evaluate(ctx, open); !res.Passed() {
		t.Fatalf("OPEN must pass: %+v", res)
	}
	closed := open
	closed.Status = entity.PeriodClosed
	if res := specification.PeriodOpen().Evaluate(ctx, closed); res.Passed() || res.Violations[0].Code != "PERIOD_CLOSED" {
		t.Fatalf("CLOSED = %+v", res)
	}
}

func TestSpecComposition(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	posting := txPosting(testPosting1, txEntries(testPosting1, 100, 100))
	combined := specification.All[entity.PostingData](
		specification.PostingBalancesPerCurrency(),
		specification.PostingTemplateAllowed(transferTemplate(), templateAccounts()),
	)
	if res := combined.Evaluate(ctx, posting); !res.Passed() {
		t.Fatalf("good posting must pass composition: %+v", res)
	}
	bad := txPosting(testPosting2, txEntries(testPosting2, 100, 90))
	bad.Operation = "payout.v1"
	res := combined.Evaluate(ctx, bad)
	if res.Passed() || len(res.Violations) != 2 {
		t.Fatalf("bad posting must aggregate both violations: %+v", res)
	}
}
