package nats_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kadekutama/go-template/internal/domain/entity"
	edgenats "github.com/kadekutama/go-template/internal/infrastructure/messaging/nats"
)

func TestEventSubject(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		eventType      string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "versioned event subject",
			tenant:         "t1",
			eventType:      "transfer.completed.v1",
			expectedResult: "ledger.t1.transfer.completed.v1",
			expectedError:  nil,
		},
		{
			name:           "empty tenant rejected",
			tenant:         "",
			eventType:      "transfer.completed.v1",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant is required",
			),
		},
		{
			name:           "unversioned event rejected",
			tenant:         "t1",
			eventType:      "transfer",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject event type must be versioned",
			),
		},
		{
			name:           "blank event rejected",
			tenant:         "t1",
			eventType:      "",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject event type is required",
			),
		},
		{
			name:           "colon in tenant rejected",
			tenant:         "t:1",
			eventType:      "transfer.completed.v1",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant must not contain '.' or ':'",
			),
		},
		{
			name:           "whitespace tenant rejected",
			tenant:         "   ",
			eventType:      "transfer.completed.v1",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant is required",
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualResult, err := edgenats.EventSubject(tc.tenant, tc.eventType)
			assert.Equal(t, tc.expectedResult, actualResult)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestParseEventSubject(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name              string
		subject           string
		expectedTenant    string
		expectedEventType string
		expectedError     error
	}

	testCases := []testCase{
		{
			name:              "valid subject parsed",
			subject:           "ledger.t1.transfer.completed.v1",
			expectedTenant:    "t1",
			expectedEventType: "transfer.completed.v1",
			expectedError:     nil,
		},
		{
			name:              "missing ledger prefix rejected",
			subject:           "other.t1.transfer.completed.v1",
			expectedTenant:    "",
			expectedEventType: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject must start with 'ledger.'",
			),
		},
		{
			name:              "too few segments rejected",
			subject:           "ledger.t1",
			expectedTenant:    "",
			expectedEventType: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject must carry tenant and event type",
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tenant, eventType, err := edgenats.ParseEventSubject(tc.subject)
			assert.Equal(t, tc.expectedTenant, tenant)
			assert.Equal(t, tc.expectedEventType, eventType)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestBalanceSubject(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		tenant         string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "valid tenant",
			tenant:         "t1",
			expectedResult: "ledger.t1." + edgenats.BalanceChangedEvent,
			expectedError:  nil,
		},
		{
			name:           "empty tenant rejected",
			tenant:         "",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant is required",
			),
		},
		{
			name:           "whitespace tenant rejected",
			tenant:         "   ",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant is required",
			),
		},
		{
			name:           "colon in tenant rejected",
			tenant:         "t:1",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant must not contain '.' or ':'",
			),
		},
		{
			name:           "dot in tenant rejected",
			tenant:         "t.1",
			expectedResult: "",
			expectedError: entity.NewError(
				"ISOLATION_SUBJECT_INVALID",
				"subject tenant must not contain '.' or ':'",
			),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subject, err := edgenats.BalanceSubject(tc.tenant)
			assert.Equal(t, tc.expectedResult, subject)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}

func TestQueueGroupFor(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		group          string
		expectedResult string
		expectedError  error
	}

	testCases := []testCase{
		{
			name:           "group maps",
			group:          "webhook-dispatcher",
			expectedResult: "q-webhook-dispatcher",
			expectedError:  nil,
		},
		{
			name:           "trims surrounding whitespace",
			group:          "  webhook-dispatcher  ",
			expectedResult: "q-webhook-dispatcher",
			expectedError:  nil,
		},
		{
			name:           "tab whitespace trimmed",
			group:          "\twebhook-dispatcher\n",
			expectedResult: "q-webhook-dispatcher",
			expectedError:  nil,
		},
		{
			name:           "blank rejected",
			group:          "",
			expectedResult: "",
			expectedError:  errors.New("nats: consumer group is required"),
		},
		{
			name:           "whitespace rejected",
			group:          "   ",
			expectedResult: "",
			expectedError:  errors.New("nats: consumer group is required"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			group, err := edgenats.QueueGroupFor(tc.group)
			assert.Equal(t, tc.expectedResult, group)
			assert.Equal(t, tc.expectedError, err)
		})
	}
}
