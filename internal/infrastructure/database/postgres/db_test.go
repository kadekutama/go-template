package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
)

func TestOpenRequiresDSN(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		dsn         string
		expectError bool
	}

	testCases := []testCase{
		{
			name:        "empty DSN rejected",
			dsn:         "",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, err := postgres.Open(postgres.Config{DSN: tc.dsn})
			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, db)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDefaultConfigPool(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		dsn             string
		expectedMaxOpen int
		expectedMaxIdle int
	}

	testCases := []testCase{
		{ //nolint:gosec // E07-T01: non-production test DSN, no real credential.
			name:            "pool defaults positive",
			dsn:             "postgres://ledger:pw@localhost:5432/ledger?sslmode=disable",
			expectedMaxOpen: 25,
			expectedMaxIdle: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := postgres.DefaultConfig(tc.dsn)
			assert.Equal(t, tc.dsn, cfg.DSN)
			assert.Equal(t, tc.expectedMaxOpen, cfg.MaxOpen)
			assert.Equal(t, tc.expectedMaxIdle, cfg.MaxIdle)
			assert.Positive(t, int64(cfg.ConnMaxLifetime))
		})
	}
}
