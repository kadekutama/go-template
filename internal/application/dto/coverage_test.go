package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/command"
	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestExtendedMapping(t *testing.T) {
	t.Parallel()

	t.Run("run summary maps counts", func(t *testing.T) {
		actual := dto.ToRunSummaryDTO(query.RunSummary{
			RunID: "run-1", Status: "COMPLETED", Total: 2, ByStatus: map[string]int{"OPEN": 1, "RESOLVED": 1},
		})
		assert.Equal(t, "run-1", actual.RunID)
		assert.Equal(t, 2, actual.Total)
		assert.Equal(t, 1, actual.ByStatus["OPEN"])
	})

	t.Run("subscription maps schedule", func(t *testing.T) {
		at := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
		actual := dto.ToSubscriptionDTO(command.Subscription{
			ID: "sub-1", TenantID: "t-1", Template: "settlement", Cron: "0 9 * * *",
			Destination: "webhook:ops", CreatedAt: at,
		})
		assert.Equal(t, "sub-1", actual.ID)
		assert.Equal(t, "settlement", actual.Template)
	})

	t.Run("volume buckets map", func(t *testing.T) {
		actual := dto.ToVolumeBuckets([]query.VolumeBucket{{Bucket: "2026-09-15", Count: 2, TotalMinor: 16000}})
		assert.Len(t, actual, 1)
		assert.Equal(t, int64(16000), actual[0].TotalMinor)
	})

	t.Run("transfer view maps", func(t *testing.T) {
		actual := dto.ToTransferViewDTO(port.TransferView{TransferID: "x-1", Status: "COMPLETED", PostingID: "p-1", Cursor: "c-1"})
		assert.Equal(t, "x-1", actual.ID)
		assert.Equal(t, "p-1", actual.PostingID)
	})

	t.Run("transaction maps posting plus link", func(t *testing.T) {
		posting := dto.ToPostingDTO(entity.PostingData{ID: "p-2", TenantID: "t-1"}, "c-1")
		actual := dto.ToTransactionDTO(posting, "p-1")
		assert.Equal(t, "p-2", actual.Posting.ID)
		assert.Equal(t, "p-1", actual.ReversalOf)
	})
}
