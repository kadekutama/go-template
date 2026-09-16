package dto_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/dto"
)

func TestPaymentRefundPayoutDTOs(t *testing.T) {
	t.Parallel()

	t.Run("intent maps fields", func(t *testing.T) {
		actual := dto.ToPaymentIntentDTO("pi-1", "t-1", "CONFIRMED", "USD", 5000, "cursor-5")
		assert.Equal(t, "pi-1", actual.ID)
		assert.Equal(t, "CONFIRMED", actual.Status)
		assert.Equal(t, int64(5000), actual.AmountMinor)
	})

	t.Run("refund maps linkage", func(t *testing.T) {
		actual := dto.ToRefundDTO("re-1", "pi-1", "SUCCEEDED", "cursor-5")
		assert.Equal(t, "re-1", actual.ID)
		assert.Equal(t, "pi-1", actual.OriginalID)
	})

	t.Run("payout maps amount", func(t *testing.T) {
		actual := dto.ToPayoutDTO("po-1", "PENDING", "USD", 5000, "cursor-5")
		assert.Equal(t, "po-1", actual.ID)
		assert.Equal(t, int64(5000), actual.AmountMinor)
	})

	t.Run("topup maps amount", func(t *testing.T) {
		actual := dto.ToTopupDTO("tp-1", "PENDING", "USD", 5000, "cursor-5")
		assert.Equal(t, "tp-1", actual.ID)
		assert.Equal(t, "PENDING", actual.Status)
	})
}
