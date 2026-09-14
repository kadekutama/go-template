package entity_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestDisputeValidate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	base := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "p-1",
		OriginalPostingID: "pst-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          now,
		EvidenceDueAt:     now.Add(14 * 24 * time.Hour),
		Status:            valueobject.DisputeOpen,
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    0,
		PolicyVersion:     "v2026.1",
	}

	type testCase struct {
		name          string
		dispute       entity.Dispute
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid dispute",
			dispute:       base,
			expectedError: nil,
		},
		{
			name: "missing id",
			dispute: func() entity.Dispute {
				d := base
				d.ID = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting"),
		},
		{
			name: "missing payment",
			dispute: func() entity.Dispute {
				d := base
				d.PaymentID = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting"),
		},
		{
			name: "missing original posting",
			dispute: func() entity.Dispute {
				d := base
				d.OriginalPostingID = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_ID_REQUIRED", "dispute requires id, payment, and original posting"),
		},
		{
			name: "missing network",
			dispute: func() entity.Dispute {
				d := base
				d.Network = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_NETWORK_REQUIRED", "dispute requires a network"),
		},
		{
			name: "zero amount",
			dispute: func() entity.Dispute {
				d := base
				d.AmountMinor = 0
				return d
			}(),
			expectedError: entity.NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive"),
		},
		{
			name: "negative amount",
			dispute: func() entity.Dispute {
				d := base
				d.AmountMinor = -10
				return d
			}(),
			expectedError: entity.NewError("INVALID_DISPUTE_AMOUNT", "dispute amount must be positive"),
		},
		{
			name: "missing opened at",
			dispute: func() entity.Dispute {
				d := base
				d.OpenedAt = time.Time{}
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_WINDOW_INVALID", "dispute requires an evidence window after opening"),
		},
		{
			name: "missing evidence due at",
			dispute: func() entity.Dispute {
				d := base
				d.EvidenceDueAt = time.Time{}
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_WINDOW_INVALID", "dispute requires an evidence window after opening"),
		},
		{
			name: "evidence due before opened",
			dispute: func() entity.Dispute {
				d := base
				d.EvidenceDueAt = now.Add(-time.Hour)
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_WINDOW_INVALID", "dispute requires an evidence window after opening"),
		},
		{
			name: "evidence due equals opened",
			dispute: func() entity.Dispute {
				d := base
				d.EvidenceDueAt = now
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_WINDOW_INVALID", "dispute requires an evidence window after opening"),
		},
		{
			name: "invalid status",
			dispute: func() entity.Dispute {
				d := base
				d.Status = "INVALID_STATUS"
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_STATUS_INVALID", "dispute status is invalid"),
		},
		{
			name: "missing hold id",
			dispute: func() entity.Dispute {
				d := base
				d.HoldID = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_HOLD_REQUIRED", "dispute requires a durable hold reference"),
		},
		{
			name: "missing policy version",
			dispute: func() entity.Dispute {
				d := base
				d.PolicyVersion = ""
				return d
			}(),
			expectedError: entity.NewError("DISPUTE_POLICY_REQUIRED", "dispute requires a versioned network policy"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := tc.dispute.Validate()
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
