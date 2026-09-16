package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestDisputeQueryServiceGetDispute(t *testing.T) {
	t.Parallel()

	openedAt := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	record := entity.Dispute{
		ID:                "d-1",
		PaymentID:         "pi-1",
		OriginalPostingID: "p-1",
		Network:           "visa",
		AmountMinor:       5000,
		OpenedAt:          openedAt,
		EvidenceDueAt:     deadline,
		Status:            valueobject.DisputeStatus("OPEN"),
		HoldID:            "h-1",
		FeeMinor:          1500,
		RepresentStage:    1,
		PolicyVersion:     "v1",
	}

	type testCase struct {
		name           string
		disputes       *stubDisputes
		query          port.DisputeQuery
		expectedResult port.DisputeResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find dispute succeeds with fee and deadline",
			disputes: &stubDisputes{
				dispute: record,
			},
			query: port.DisputeQuery{
				DisputeID: "d-1",
			},
			expectedResult: port.DisputeResult{
				Dispute: record,
			},
			expectedError: nil,
		},
		{
			name: "dispute not found propagates error",
			disputes: &stubDisputes{
				err: entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown"),
			},
			query: port.DisputeQuery{
				DisputeID: "d-missing",
			},
			expectedResult: port.DisputeResult{},
			expectedError:  entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewDisputeQueryService(query.DisputeQueryServiceParams{Disputes: tc.disputes})
			actualResult, err := svc.GetDispute(context.Background(), tc.query)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestDisputeQueryServiceListDisputes(t *testing.T) {
	t.Parallel()

	openedAt := time.Date(2026, 9, 15, 12, 0, 0, 0, time.UTC)
	deadline := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	records := []entity.Dispute{
		{
			ID:                "d-1",
			PaymentID:         "pi-1",
			OriginalPostingID: "p-1",
			Network:           "visa",
			AmountMinor:       5000,
			OpenedAt:          openedAt,
			EvidenceDueAt:     deadline,
			Status:            valueobject.DisputeStatus("OPEN"),
			HoldID:            "h-1",
			FeeMinor:          1500,
			RepresentStage:    1,
			PolicyVersion:     "v1",
		},
	}

	type testCase struct {
		name           string
		disputes       *stubDisputes
		filter         port.DisputeListFilter
		expectedResult port.DisputePage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "list disputes returns page",
			disputes: &stubDisputes{
				disputes: records,
				next:     "cursor-next",
			},
			filter: port.DisputeListFilter{
				Status: "OPEN",
				Limit:  10,
			},
			expectedResult: port.DisputePage{
				Disputes: []port.DisputeResult{
					{
						Dispute: records[0],
					},
				},
				NextCursor: "cursor-next",
			},
			expectedError: nil,
		},
		{
			name: "store error propagates",
			disputes: &stubDisputes{
				err: entity.NewError("DISPUTE_FILTER_INVALID", "dispute filter is invalid"),
			},
			filter: port.DisputeListFilter{
				Status: "BOGUS",
			},
			expectedResult: port.DisputePage{},
			expectedError:  entity.NewError("DISPUTE_FILTER_INVALID", "dispute filter is invalid"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc := query.NewDisputeQueryService(query.DisputeQueryServiceParams{Disputes: tc.disputes})
			actualResult, err := svc.ListDisputes(context.Background(), tc.filter)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
