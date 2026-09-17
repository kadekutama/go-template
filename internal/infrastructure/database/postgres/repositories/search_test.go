package repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
)

func TestParseSeqCursor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		cursor         string
		expectedResult int64
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "empty starts at zero",
			cursor:         "",
			expectedResult: 0,
			expectedError:  nil,
		},
		{
			name:           "decimal decodes",
			cursor:         "42",
			expectedResult: 42,
			expectedError:  nil,
		},
		{
			name:           "entry id rejected",
			cursor:         "550e8400-e29b-41d4-a716-446655440000",
			expectedResult: 0,
			expectedError:  errors.New("postgres: invalid cursor"),
		},
		{
			name:           "negative rejected",
			cursor:         "-1",
			expectedResult: 0,
			expectedError:  errors.New("postgres: invalid cursor"),
		},
		{
			name:           "garbage rejected",
			cursor:         "next-page",
			expectedResult: 0,
			expectedError:  errors.New("postgres: invalid cursor"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := parseSeqCursor(tc.cursor)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewPostingSearch(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		params         PostingSearchParams
		expectedResult *PostingSearch
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "nil DB rejected",
			params: PostingSearchParams{
				DB: nil,
			},
			expectedResult: nil,
			expectedError:  errors.New("postgres: posting search needs DB"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := NewPostingSearch(tc.params)
			assert.Equal(t, tc.expectedResult, actualResult)
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Equal(t, tc.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPostingSearchGuards(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		ctx            context.Context
		filter         appport.PostingFilter
		expectedResult appport.PostingSearchPage
		expectedError  error
	}

	testCases := []testCase{
		{
			name: "missing tenant fails closed",
			ctx:  context.Background(),
			filter: appport.PostingFilter{
				TenantID: "",
			},
			expectedResult: appport.PostingSearchPage{},
			expectedError:  errors.New("postgres: tenant is required"),
		},
		{
			name: "amount bounds need the reporting read model",
			ctx:  context.Background(),
			filter: appport.PostingFilter{
				TenantID:  valueobject.TenantID("tnt-01"),
				MinAmount: 100,
			},
			expectedResult: appport.PostingSearchPage{},
			expectedError:  errors.New("postgres: amount bounds need the reporting read model (E06-T09)"),
		},
		{
			name: "status filter rejected with reason",
			ctx:  context.Background(),
			filter: appport.PostingFilter{
				TenantID: valueobject.TenantID("tnt-01"),
				Status:   "PENDING",
			},
			expectedResult: appport.PostingSearchPage{},
			expectedError:  errors.New("postgres: postings carry no status; corrections are new linked postings"),
		},
		{
			name: "bad cursor rejected before SQL",
			ctx:  context.Background(),
			filter: appport.PostingFilter{
				TenantID: valueobject.TenantID("tnt-01"),
				Cursor:   "not-a-seq",
			},
			expectedResult: appport.PostingSearchPage{},
			expectedError:  errors.New("postgres: invalid cursor"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			search := &PostingSearch{db: nil}
			actualResult, err := search.Search(tc.ctx, tc.filter)
			assert.Equal(t, tc.expectedResult, actualResult)
			require.Error(t, err)
			assert.Equal(t, tc.expectedError.Error(), err.Error())
		})
	}
}
