package port_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/entity"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	mockdomain "github.com/kadekutama/go-template/test/mock/domain"
)

func TestTransferUseCasesMock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		programmedResult port.TransferResult
		programmedError  error
		expectedResult   port.TransferResult
		expectedError    error
	}

	testCases := []testCase{
		{
			name: "programmed success verifies once",
			programmedResult: port.TransferResult{
				TransferID: "x-1",
				Status:     "COMPLETED",
				Cursor:     "cursor-1",
			},
			programmedError: nil,
			expectedResult: port.TransferResult{
				TransferID: "x-1",
				Status:     "COMPLETED",
				Cursor:     "cursor-1",
			},
			expectedError: nil,
		},
		{
			name:             "programmed failure propagates",
			programmedResult: port.TransferResult{},
			programmedError:  errors.New("processor down"),
			expectedResult:   port.TransferResult{},
			expectedError:    errors.New("processor down"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			transfers := new(mockapplication.MockTransferUseCases)
			transfers.On("CreateTransfer", mock.Anything, mock.Anything).Return(tc.programmedResult, tc.programmedError).Once()
			actualResult, err := transfers.CreateTransfer(context.Background(), port.TransferRequest{
				TenantID:    "t-1",
				Source:      "a-src",
				Dest:        "a-dst",
				AssetCode:   "USD",
				AmountMinor: 5000,
			})
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
			transfers.AssertExpectations(t)
		})
	}
}

func TestPostingRepositoryMock(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		programmedError error
		expectedError   error
	}

	testCases := []testCase{
		{
			name:            "commit success records once",
			programmedError: nil,
			expectedError:   nil,
		},
		{
			name:            "commit failure propagates",
			programmedError: entity.NewError("POSTING_CONFLICT", "posting id already committed"),
			expectedError:   entity.NewError("POSTING_CONFLICT", "posting id already committed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			postings := new(mockdomain.MockPostingRepository)
			postings.On("Commit", mock.Anything, mock.Anything).Return(tc.programmedError).Once()
			err := postings.Commit(context.Background(), entity.PostingData{ID: valueobject.PostingID("p-1")})
			assert.Equal(t, tc.expectedError, err)
			postings.AssertExpectations(t)
		})
	}
}
