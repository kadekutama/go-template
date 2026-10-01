package redpanda_test

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/messaging/redpanda"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        redpanda.RedpandaParams
		expectedSeeds []string
		expectedTLS   bool
		expectedSASL  bool
		expectedError error
	}

	testCases := []testCase{
		{
			name: "single seed",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: []string{"redpanda-1:9092"},
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: nil,
		},
		{
			name: "blank seeds filtered",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"  ", "redpanda-1:9092", ""},
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: []string{"redpanda-1:9092"},
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: nil,
		},
		{
			name: "tls and sasl flags enabled",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				UseTLS:          true,
				SASLUser:        "app",
				SASLPass:        "secret",
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: []string{"redpanda-1:9092"},
			expectedTLS:   true,
			expectedSASL:  true,
			expectedError: nil,
		},
		{
			name: "tls only without sasl",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				UseTLS:          true,
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: []string{"redpanda-1:9092"},
			expectedTLS:   true,
			expectedSASL:  false,
			expectedError: nil,
		},
		{
			name: "no seeds rejected",
			params: redpanda.RedpandaParams{
				Seeds:           nil,
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: errors.New("redpanda: at least one seed is required"),
		},
		{
			name: "all blank rejected",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"", "  "},
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: errors.New("redpanda: at least one seed is required"),
		},
		{
			name: "negative dial timeout rejected",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				DialTimeout:     -time.Second,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: errors.New("redpanda: invalid params (1 violation(s)): DialTimeout: rule \"gt\" on value -1s"),
		},
		{
			name: "missing dial timeout rejected",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				DialTimeout:     0,
				MetadataTimeout: 5 * time.Second,
			},
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: errors.New("redpanda: invalid params (1 violation(s)): DialTimeout: rule \"required\" on value 0s"),
		},
		{
			name: "missing metadata timeout rejected",
			params: redpanda.RedpandaParams{
				Seeds:           []string{"redpanda-1:9092"},
				DialTimeout:     5 * time.Second,
				MetadataTimeout: 0,
			},
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
			expectedError: errors.New("redpanda: invalid params (1 violation(s)): MetadataTimeout: rule \"required\" on value 0s"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client, err := redpanda.NewClient(tc.params)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				assert.Nil(t, client)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, client)
			assert.Equal(t, tc.expectedSeeds, client.Seeds())
			assert.Equal(t, tc.expectedTLS, client.UsesTLS())
			assert.Equal(t, tc.expectedSASL, client.HasSASL())
		})
	}
}

func TestClientNilSafety(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		client        *redpanda.Client
		expectedSeeds []string
		expectedTLS   bool
		expectedSASL  bool
	}

	testCases := []testCase{
		{
			name:          "nil client methods return safe defaults",
			client:        nil,
			expectedSeeds: nil,
			expectedTLS:   false,
			expectedSASL:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedSeeds, tc.client.Seeds())
			assert.Equal(t, tc.expectedTLS, tc.client.UsesTLS())
			assert.Equal(t, tc.expectedSASL, tc.client.HasSASL())
		})
	}
}
