package jwt_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/jwt"
	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	"github.com/kadekutama/go-template/test/testcontainers"
)

const refreshTenant = "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9e"

func openReceiptsDB(t *testing.T) *gorm.DB {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartPostgres(t, "ledger_refresh")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", handle.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	runner, err := migration.NewRunner(migration.RunnerParams{
		DB:      sqlDB,
		Timeout: 30 * time.Second,
	})
	require.NoError(t, err)
	require.NoError(t, runner.Up(context.Background()))

	gormDB, err := postgres.Open(postgres.Config{
		DSN:             handle.ConnectionString(),
		MaxOpen:         25,
		MaxIdle:         5,
		ConnMaxLifetime: 30 * time.Minute,
	})
	require.NoError(t, err)

	return gormDB
}

func openReceipts(t *testing.T, clk appport.Clock) *jwt.ReceiptStore {
	t.Helper()

	store, err := jwt.NewRefreshStore(jwt.ReceiptParams{
		DB:    openReceiptsDB(t),
		TTL:   7 * 24 * time.Hour,
		Clock: clk,
	})
	require.NoError(t, err)

	return store
}

func TestRefreshValidation(t *testing.T) {
	t.Parallel()

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		params        jwt.ReceiptParams
		expectedError error
	}

	testCases := []testCase{
		{
			name: "missing db rejected",
			params: func() jwt.ReceiptParams {
				return jwt.ReceiptParams{TTL: time.Hour, Clock: clk}
			}(),
			expectedError: jwt.ErrConfigRequired,
		},
		{
			name: "missing clock rejected",
			params: func() jwt.ReceiptParams {
				db, err := gorm.Open(gormpostgres.Open("postgres://127.0.0.1:1/jwt_unit?sslmode=disable"), &gorm.Config{
					SkipDefaultTransaction: true,
					DisableAutomaticPing:   true,
				})
				if err != nil {
					panic(err)
				}

				return jwt.ReceiptParams{DB: db, TTL: time.Hour}
			}(),
			expectedError: jwt.ErrClockRequired,
		},
		{
			name: "zero ttl rejected",
			params: func() jwt.ReceiptParams {
				return jwt.ReceiptParams{Clock: clk}
			}(),
			expectedError: jwt.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jwt.NewRefreshStore(tc.params)
			assert.ErrorIs(t, err, tc.expectedError)
		})
	}
}

func TestRefreshRotationDurable(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		subject       string
		tenant        string
		replay        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "single spend succeeds",
			subject:       "user-1",
			tenant:        refreshTenant,
			replay:        false,
			expectedError: nil,
		},
		{
			name:          "replay revokes family",
			subject:       "user-2",
			tenant:        refreshTenant,
			replay:        true,
			expectedError: jwt.ErrRefreshReuse,
		},
		{
			name:          "empty subject rejected",
			subject:       "",
			tenant:        refreshTenant,
			replay:        false,
			expectedError: jwt.ErrInvalidClaims,
		},
		{
			name:          "non-uuid tenant rejected",
			subject:       "user-3",
			tenant:        "not-a-uuid",
			replay:        false,
			expectedError: jwt.ErrInvalidClaims,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := openReceipts(t, clk)

			_, token, err := store.IssueRefresh(context.Background(), tc.subject, tc.tenant)
			if tc.subject == "" || tc.tenant == "not-a-uuid" {
				assert.ErrorIs(t, err, jwt.ErrInvalidClaims)
				return
			}
			require.NoError(t, err)

			claim, err := store.UseRefresh(context.Background(), token)
			require.NoError(t, err)
			assert.Equal(t, tc.subject, claim.Subject)
			assert.NotEmpty(t, claim.NextToken)

			if tc.replay {
				_, err = store.UseRefresh(context.Background(), token)
				assert.ErrorIs(t, err, jwt.ErrRefreshReuse)

				_, err = store.UseRefresh(context.Background(), token)
				assert.ErrorIs(t, err, jwt.ErrRefreshRevoked)
			} else {
				secondClaim, err := store.UseRefresh(context.Background(), claim.NextToken)
				require.NoError(t, err)
				assert.Equal(t, tc.subject, secondClaim.Subject)
				assert.NotEmpty(t, secondClaim.NextToken)
				assert.NotEqual(t, claim.NextToken, secondClaim.NextToken)
			}
		})
	}
}

func TestRefreshUnknownToken(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		token         string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "garbage rejected",
			token:         "nope",
			expectedError: jwt.ErrRefreshNotFound,
		},
		{
			name:          "canceled context",
			token:         "rt_x." + refreshTenant + ".deadbeef",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			store := openReceipts(t, clk)

			ctx := context.Background()
			if tc.name == "canceled context" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			_, err := store.UseRefresh(ctx, tc.token)
			assert.Error(t, err)
		})
	}
}

func TestRefreshStaleFamilyExpires(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	base := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(base).Maybe()

	db := openReceiptsDB(t)
	issuer, err := jwt.NewRefreshStore(jwt.ReceiptParams{DB: db, TTL: time.Hour, Clock: clk})
	require.NoError(t, err)

	_, token, err := issuer.IssueRefresh(context.Background(), "user-1", refreshTenant)
	require.NoError(t, err)

	laterClk := mockapplication.NewMockClock(t)
	laterClk.EXPECT().Now().Return(base.Add(2 * time.Hour)).Maybe()
	spender, err := jwt.NewRefreshStore(jwt.ReceiptParams{DB: db, TTL: time.Hour, Clock: laterClk})
	require.NoError(t, err)

	_, err = spender.UseRefresh(context.Background(), token)
	assert.ErrorIs(t, err, jwt.ErrExpired)

	_, err = spender.UseRefresh(context.Background(), token)
	assert.ErrorIs(t, err, jwt.ErrRefreshRevoked)
}

func TestRefreshUseCanceledContext(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	store := openReceipts(t, clk)

	_, token, err := store.IssueRefresh(context.Background(), "user-1", refreshTenant)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.UseRefresh(ctx, token)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
