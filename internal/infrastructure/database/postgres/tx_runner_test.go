package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"

	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
)

func TestWithinTxValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		db            *gorm.DB
		fn            func(tx *gorm.DB) error
		expectedError error
	}

	testCases := []testCase{
		{
			name: "nil db rejected",
			ctx:  context.Background(),
			db:   nil,
			fn: func(_ *gorm.DB) error {
				return nil
			},
			expectedError: errors.New("postgres: db is required"),
		},
		{
			name:          "nil fn rejected",
			ctx:           context.Background(),
			db:            &gorm.DB{},
			fn:            nil,
			expectedError: errors.New("postgres: transaction callback is required"),
		},
		{
			name: "canceled context rejected",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			db: &gorm.DB{},
			fn: func(_ *gorm.DB) error {
				return nil
			},
			expectedError: errors.New("postgres: context canceled: context canceled"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := postgres.WithinTx(tc.ctx, tc.db, tc.fn)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWithinTenantTxValidation(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		ctx           context.Context
		db            *gorm.DB
		tenant        valueobject.TenantID
		fn            func(tx *gorm.DB) error
		expectedError error
	}

	testCases := []testCase{
		{
			name:   "nil db rejected",
			ctx:    context.Background(),
			db:     nil,
			tenant: "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9d",
			fn: func(_ *gorm.DB) error {
				return nil
			},
			expectedError: errors.New("postgres: db is required"),
		},
		{
			name:          "nil fn rejected",
			ctx:           context.Background(),
			db:            &gorm.DB{},
			tenant:        "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9d",
			fn:            nil,
			expectedError: errors.New("postgres: transaction callback is required"),
		},
		{
			name:   "empty tenant rejected",
			ctx:    context.Background(),
			db:     &gorm.DB{},
			tenant: "",
			fn: func(_ *gorm.DB) error {
				return nil
			},
			expectedError: errors.New("postgres: tenant is required"),
		},
		{
			name: "canceled context rejected",
			ctx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			}(),
			db:     &gorm.DB{},
			tenant: "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9d",
			fn: func(_ *gorm.DB) error {
				return nil
			},
			expectedError: errors.New("postgres: context canceled: context canceled"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := postgres.WithinTenantTx(tc.ctx, tc.db, tc.tenant, tc.fn)
			if tc.expectedError != nil {
				assert.EqualError(t, err, tc.expectedError.Error())
				return
			}
			assert.NoError(t, err)
		})
	}
}
