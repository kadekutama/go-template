package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/application/query"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockcommand "github.com/kadekutama/go-template/test/mock/command"
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
		query          port.DisputeQuery
		programmed     entity.Dispute
		programmedErr  error
		expectedResult port.DisputeResult
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "find dispute succeeds with fee and deadline",
			query: port.DisputeQuery{
				DisputeID: "d-1",
			},
			programmed:    record,
			programmedErr: nil,
			expectedResult: port.DisputeResult{
				Dispute: record,
			},
			expectedError: nil,
		},
		{
			name: "dispute not found propagates error",
			query: port.DisputeQuery{
				DisputeID: "d-missing",
			},
			programmed:     entity.Dispute{},
			programmedErr:  entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown"),
			expectedResult: port.DisputeResult{},
			expectedError:  entity.NewError("DISPUTE_NOT_FOUND", "dispute is unknown"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			disputes := mockcommand.NewMockDisputeStore(t)
			disputes.EXPECT().
				FindDispute(mock.Anything, tc.query.DisputeID).
				Return(tc.programmed, tc.programmedErr).
				Once()

			svc := query.NewDisputeQueryService(query.DisputeQueryServiceParams{Disputes: disputes})
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
		filter         port.DisputeListFilter
		programmed     []entity.Dispute
		programmedNext string
		programmedErr  error
		expectedResult port.DisputePage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "list disputes returns page",
			filter: port.DisputeListFilter{
				Status: "OPEN",
				Limit:  10,
			},
			programmed:     records,
			programmedNext: "cursor-next",
			programmedErr:  nil,
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
			filter: port.DisputeListFilter{
				Status: "BOGUS",
				Limit:  10,
			},
			programmed:     nil,
			programmedNext: "",
			programmedErr:  entity.NewError("DISPUTE_FILTER_INVALID", "dispute filter is invalid"),
			expectedResult: port.DisputePage{},
			expectedError:  entity.NewError("DISPUTE_FILTER_INVALID", "dispute filter is invalid"),
		},
		{
			name: "non-positive limit returns validation error",
			filter: port.DisputeListFilter{
				Status: "OPEN",
				Limit:  0,
			},
			programmed:     nil,
			programmedNext: "",
			programmedErr:  nil,
			expectedResult: port.DisputePage{},
			expectedError:  entity.NewError("INVALID_PAGE_LIMIT", "limit must be between 1 and 100"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			disputes := mockcommand.NewMockDisputeStore(t)
			if tc.filter.Limit > 0 && tc.filter.Limit <= 100 {
				disputes.EXPECT().
					ListDisputes(mock.Anything, tc.filter.Status, tc.filter.From, tc.filter.To, tc.filter.Cursor, tc.filter.Limit).
					Return(tc.programmed, tc.programmedNext, tc.programmedErr).
					Once()
			}

			svc := query.NewDisputeQueryService(query.DisputeQueryServiceParams{Disputes: disputes})
			actualResult, err := svc.ListDisputes(context.Background(), tc.filter)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
