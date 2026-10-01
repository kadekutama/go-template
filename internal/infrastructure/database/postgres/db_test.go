package postgres_test

import (
	"errors"
	"testing"
	"time"

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
			db, err := postgres.Open(postgres.Config{
				DSN:             tc.dsn,
				MaxOpen:         25,
				MaxIdle:         5,
				ConnMaxLifetime: 30 * time.Minute,
			})
			if tc.expectError {
				require.Error(t, err)
				assert.Nil(t, db)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	baseCfg := postgres.Config{
		DSN:             "postgres://postgres@localhost:5432/ledger?sslmode=disable",
		MaxOpen:         25,
		MaxIdle:         5,
		ConnMaxLifetime: 30 * time.Minute,
	}

	type testCase struct {
		name          string
		cfg           postgres.Config
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "valid config",
			cfg:           baseCfg,
			expectedError: nil,
		},
		{
			name: "missing DSN",
			cfg: func() postgres.Config {
				c := baseCfg
				c.DSN = ""
				return c
			}(),
			expectedError: errors.New("postgres: DSN is required"),
		},
		{
			name: "non-positive max open",
			cfg: func() postgres.Config {
				c := baseCfg
				c.MaxOpen = 0
				return c
			}(),
			expectedError: errors.New("postgres: MaxOpen must be positive"),
		},
		{
			name: "non-positive max idle",
			cfg: func() postgres.Config {
				c := baseCfg
				c.MaxIdle = 0
				return c
			}(),
			expectedError: errors.New("postgres: MaxIdle must be positive"),
		},
		{
			name: "non-positive conn max lifetime",
			cfg: func() postgres.Config {
				c := baseCfg
				c.ConnMaxLifetime = 0
				return c
			}(),
			expectedError: errors.New("postgres: ConnMaxLifetime must be positive"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			require.NoError(t, err)
		})
	}
}
