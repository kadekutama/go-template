package apikey_test

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/apikey"
	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	mockapplication "github.com/kadekutama/go-template/test/mock/application"
	"github.com/kadekutama/go-template/test/testcontainers"
)

const testTenant = "0199a5a0-2b7e-7a1e-9b0c-4d5e6f7a8b9c"

func openManager(t *testing.T, clk appport.Clock) (*apikey.Manager, valueobject.TenantID, *gorm.DB, *testcontainers.PostgresHandle) {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartPostgres(t, "ledger_apikey")
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

	hasher, err := apikey.NewHasher(apikey.HasherParams{
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
		SaltLen: 16,
	})
	require.NoError(t, err)

	manager, err := apikey.NewManager(apikey.APIKeyParams{
		DB:             gormDB,
		Hasher:         hasher,
		RotationWindow: time.Hour,
		Clock:          clk,
	})
	require.NoError(t, err)

	tenant, err := valueobject.ParseTenantID(testTenant)
	require.NoError(t, err)

	return manager, tenant, gormDB, handle
}

// openManagerOnDB rebuilds a Manager over an existing database handle (for
// clock-advanced or role-restricted scenarios sharing one migrated database).
func openManagerOnDB(t *testing.T, gormDB *gorm.DB, clk appport.Clock, window time.Duration) (*apikey.Manager, valueobject.TenantID) {
	t.Helper()

	hasher, err := apikey.NewHasher(apikey.HasherParams{
		Time:    3,
		Memory:  64 * 1024,
		Threads: 4,
		KeyLen:  32,
		SaltLen: 16,
	})
	require.NoError(t, err)

	manager, err := apikey.NewManager(apikey.APIKeyParams{
		DB:             gormDB,
		Hasher:         hasher,
		RotationWindow: window,
		Clock:          clk,
	})
	require.NoError(t, err)

	tenant, err := valueobject.ParseTenantID(testTenant)
	require.NoError(t, err)

	return manager, tenant
}

func TestAPIKeyDurableLifecycle(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		keyName       string
		scopes        []string
		ttl           time.Duration
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "create and verify durable",
			keyName:       "server-1",
			scopes:        []string{"ledger:read"},
			ttl:           24 * time.Hour,
			expectedError: nil,
		},
		{
			name:          "empty name rejected",
			keyName:       "",
			scopes:        nil,
			ttl:           24 * time.Hour,
			expectedError: apikey.ErrConfigRequired,
		},
		{
			name:          "zero ttl rejected",
			keyName:       "server-2",
			scopes:        nil,
			ttl:           0,
			expectedError: apikey.ErrConfigRequired,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, tenant, _, _ := openManager(t, clk)

			secret, err := manager.Create(context.Background(), tenant, tc.keyName, tc.scopes, tc.ttl)
			if tc.expectedError != nil {
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			assert.NotEmpty(t, secret.Plaintext)
			assert.Contains(t, secret.Plaintext, string(tenant))

			got, err := manager.Verify(context.Background(), secret.Plaintext)
			require.NoError(t, err)
			assert.Equal(t, secret.Key.ID, got.ID)
			assert.True(t, apikey.HasScope(got, "ledger:read"))
			assert.False(t, apikey.HasScope(got, "ledger:write"))
		})
	}
}

func TestAPIKeyRotationRevokeDurable(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		revoke        bool
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "rotation keeps old valid",
			revoke:        false,
			expectedError: nil,
		},
		{
			name:          "revoked key fails closed",
			revoke:        true,
			expectedError: apikey.ErrRevoked,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, tenant, _, _ := openManager(t, clk)

			old, err := manager.Create(context.Background(), tenant, "rot", []string{"ledger:read"}, 24*time.Hour)
			require.NoError(t, err)

			next, err := manager.Rotate(context.Background(), tenant, old.Key.ID, 24*time.Hour)
			require.NoError(t, err)

			_, err = manager.Verify(context.Background(), old.Plaintext)
			require.NoError(t, err)

			_, err = manager.Verify(context.Background(), next.Plaintext)
			require.NoError(t, err)

			if tc.revoke {
				require.NoError(t, manager.Revoke(context.Background(), tenant, old.Key.ID))

				_, err = manager.Verify(context.Background(), old.Plaintext)
				assert.ErrorIs(t, err, apikey.ErrRevoked)
			}
		})
	}
}

func TestAPIKeyRejectsBadPlaintext(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name          string
		plaintext     string
		expectedError error
	}

	testCases := []testCase{
		{
			name:          "missing prefix",
			plaintext:     "nope",
			expectedError: apikey.ErrInvalidFormat,
		},
		{
			name:          "bad checksum",
			plaintext:     "ak_abc." + testTenant + ".def.00000000",
			expectedError: apikey.ErrInvalidFormat,
		},
		{
			name:          "non-uuid tenant",
			plaintext:     "ak_abc.not-a-uuid.def.00000000",
			expectedError: apikey.ErrInvalidFormat,
		},
		{
			name:          "canceled context",
			plaintext:     "",
			expectedError: context.Canceled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, tenant, _, _ := openManager(t, clk)

			plaintext := tc.plaintext
			ctx := context.Background()

			if tc.name == "canceled context" {
				secret, err := manager.Create(context.Background(), tenant, "ctx", nil, time.Hour)
				require.NoError(t, err)
				plaintext = secret.Plaintext

				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(context.Background())
				cancel()
			}

			_, err := manager.Verify(ctx, plaintext)
			assert.Error(t, err)
		})
	}
}

func TestAPIKeyRotationWindowExpiry(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	base := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(base).Maybe()

	type testCase struct {
		name           string
		advance        time.Duration
		expectOldError error
		expectNewError error
	}

	testCases := []testCase{
		{
			name:           "within window old key verifies",
			advance:        30 * time.Minute,
			expectOldError: nil,
			expectNewError: nil,
		},
		{
			name:           "past window old key is revoked",
			advance:        2 * time.Hour,
			expectOldError: apikey.ErrRevoked,
			expectNewError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, tenant, db, _ := openManager(t, clk)

			old, err := manager.Create(context.Background(), tenant, "rot", []string{"ledger:read"}, 24*time.Hour)
			require.NoError(t, err)

			next, err := manager.Rotate(context.Background(), tenant, old.Key.ID, 24*time.Hour)
			require.NoError(t, err)

			laterClk := mockapplication.NewMockClock(t)
			laterClk.EXPECT().Now().Return(base.Add(tc.advance)).Maybe()
			later, _ := openManagerOnDB(t, db, laterClk, time.Hour)

			_, err = later.Verify(context.Background(), old.Plaintext)
			if tc.expectOldError != nil {
				assert.ErrorIs(t, err, tc.expectOldError)
			} else {
				assert.NoError(t, err)
			}

			_, err = later.Verify(context.Background(), next.Plaintext)
			if tc.expectNewError != nil {
				assert.ErrorIs(t, err, tc.expectNewError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAPIKeyRevokeIdempotent(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	type testCase struct {
		name        string
		revocations int
	}

	testCases := []testCase{
		{
			name:        "single revoke disables key",
			revocations: 1,
		},
		{
			name:        "double revoke stays disabled without error",
			revocations: 2,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, tenant, _, _ := openManager(t, clk)

			secret, err := manager.Create(context.Background(), tenant, "rev", []string{"ledger:read"}, 24*time.Hour)
			require.NoError(t, err)

			for i := 0; i < tc.revocations; i++ {
				assert.NoError(t, manager.Revoke(context.Background(), tenant, secret.Key.ID))
			}

			_, err = manager.Verify(context.Background(), secret.Plaintext)
			assert.ErrorIs(t, err, apikey.ErrRevoked)
		})
	}
}

func TestAPIKeyVerifyEnforcesRLSForAppRole(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	manager, tenant, db, handle := openManager(t, clk)

	secret, err := manager.Create(context.Background(), tenant, "rls", []string{"ledger:read"}, 24*time.Hour)
	require.NoError(t, err)

	require.NoError(t, db.Exec("CREATE ROLE app_test WITH LOGIN PASSWORD 'app_test_pw'").Error)
	require.NoError(t, db.Exec("GRANT USAGE ON SCHEMA public TO app_test").Error)
	require.NoError(t, db.Exec("GRANT ALL ON TABLE api_keys TO app_test").Error)

	appDSN, err := appRoleDSN(t, handle.ConnectionString())
	require.NoError(t, err)
	appDB, err := postgres.Open(postgres.Config{
		DSN:             appDSN,
		MaxOpen:         5,
		MaxIdle:         1,
		ConnMaxLifetime: time.Minute,
	})
	require.NoError(t, err)

	// Belt and braces: without any tenant GUC, the RLS-subject role must see
	// zero rows even though two keys exist — proving FORCE RLS is active.
	var visible int64
	require.NoError(t, appDB.WithContext(context.Background()).Raw("SELECT count(*) FROM api_keys").Scan(&visible).Error)
	assert.Equal(t, int64(0), visible)

	appManager, _ := openManagerOnDB(t, appDB, clk, time.Hour)

	got, err := appManager.Verify(context.Background(), secret.Plaintext)
	require.NoError(t, err)
	assert.Equal(t, secret.Key.ID, got.ID)

	// Unknown prefixes stay not-found under the RLS-subject role too: the
	// negative path works identically with the GUC set.
	parts := strings.Split(strings.TrimPrefix(secret.Plaintext, "ak_"), ".")
	require.Len(t, parts, 4)
	require.NoError(t, db.Exec("DELETE FROM api_keys WHERE prefix = ?", parts[0]).Error)

	_, err = appManager.Verify(context.Background(), secret.Plaintext)
	assert.ErrorIs(t, err, apikey.ErrNotFound)
}

func appRoleDSN(t *testing.T, superDSN string) (string, error) {
	t.Helper()

	parsed, err := url.Parse(superDSN)
	if err != nil {
		return "", err
	}
	parsed.User = url.UserPassword("app_test", "app_test_pw")
	return parsed.String(), nil
}

func TestAPIKeyVerifyPersistsHashUpgrade(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	clk := mockapplication.NewMockClock(t)
	clk.EXPECT().Now().Return(time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)).Maybe()

	v1params := apikey.HasherParams{Time: 3, Memory: 64 * 1024, Threads: 4, KeyLen: 32, SaltLen: 16}
	v2params := apikey.HasherParams{Time: 4, Memory: 64 * 1024, Threads: 4, KeyLen: 32, SaltLen: 16}

	oldHasher, err := apikey.NewHasher(v1params)
	require.NoError(t, err)
	newHasher, err := apikey.NewHasherWithHistory(2, v2params, map[int]apikey.HasherParams{1: v1params})
	require.NoError(t, err)

	_, tenant, db, _ := openManager(t, clk)

	oldManager, err := apikey.NewManager(apikey.APIKeyParams{
		DB:             db,
		Hasher:         oldHasher,
		RotationWindow: time.Hour,
		Clock:          clk,
	})
	require.NoError(t, err)

	secret, err := oldManager.Create(context.Background(), tenant, "upg", []string{"ledger:read"}, 24*time.Hour)
	require.NoError(t, err)
	assertStoredVersion(t, db, 1)

	newManager, err := apikey.NewManager(apikey.APIKeyParams{
		DB:             db,
		Hasher:         newHasher,
		RotationWindow: time.Hour,
		Clock:          clk,
	})
	require.NoError(t, err)

	got, err := newManager.Verify(context.Background(), secret.Plaintext)
	require.NoError(t, err)
	assert.Equal(t, secret.Key.ID, got.ID)
	assertStoredVersion(t, db, 2)
}

func assertStoredVersion(t *testing.T, db *gorm.DB, expected int) {
	t.Helper()

	var version int
	require.NoError(t, db.WithContext(context.Background()).Raw("SELECT version FROM api_keys").Scan(&version).Error)
	assert.Equal(t, expected, version)
}
