package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/dto"
	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
)

func TestToDisputeDTO(t *testing.T) {
	t.Parallel()

	due := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)

	type testCase struct {
		name           string
		result         port.DisputeResult
		outcome        string
		expectedResult dto.DisputeDTO
	}

	testCases := []testCase{
		{
			name: "dispute maps deadline fee and outcome",
			result: port.DisputeResult{
				Dispute: entity.Dispute{
					ID: "d-1", PaymentID: "pay-1", Network: "VISA", AmountMinor: 50000,
					Status: "LOST", EvidenceDueAt: due, FeeMinor: 1500, RepresentStage: 1,
				},
				Cursor: "cursor-2",
			},
			outcome: "LOST",
			expectedResult: dto.DisputeDTO{
				ID: "d-1", PaymentID: "pay-1", Network: "VISA", AmountMinor: 50000,
				Status: "LOST", EvidenceDueAt: due, FeeMinor: 1500, Stage: 1,
				Outcome: "LOST", Cursor: "cursor-2",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedResult, dto.ToDisputeDTO(tc.result, tc.outcome))
		})
	}
}
