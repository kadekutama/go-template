package rbac_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	_ "github.com/jackc/pgx/v5/stdlib"

	appport "github.com/kadekutama/go-template/internal/application/port"
	"github.com/kadekutama/go-template/internal/domain/valueobject"
	"github.com/kadekutama/go-template/internal/infrastructure/auth/rbac"
	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/infrastructure/database/postgres"
	"github.com/kadekutama/go-template/test/testcontainers"
)

func openPolicyDB(t *testing.T) *gorm.DB {
	t.Helper()
	testcontainers.SkipIfNoDocker(t)

	handle, err := testcontainers.StartPostgres(t, "ledger_rbac")
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

func seedCasbinRows(t *testing.T, db *gorm.DB) {
	t.Helper()

	rows := []struct {
		ptype              string
		v0, v1, v2, v3, v4 string
	}{
		{"p", "admin", "tenant-acme", "ledger", "write", "allow"},
		{"p", "viewer", "tenant-acme", "ledger", "read", "allow"},
		{"g", "alice", "admin", "tenant-acme", "", ""},
		{"g", "bob", "viewer", "tenant-acme", "", ""},
	}

	for _, row := range rows {
		require.NoError(t, db.WithContext(context.Background()).Exec(
			"INSERT INTO casbin_rules (ptype, v0, v1, v2, v3, v4) VALUES (?, ?, ?, ?, ?, ?)",
			row.ptype, row.v0, row.v1, row.v2, row.v3, row.v4,
		).Error)
	}
}

func TestDBPolicyLoaderRoundTrip(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	type testCase struct {
		name             string
		seed             bool
		expectedPolicies []rbac.PolicyRule
		expectedGrouping []rbac.GroupingRule
		expectedError    error
	}

	testCases := []testCase{
		{
			name: "seeded rows map to rules",
			seed: true,
			expectedPolicies: []rbac.PolicyRule{
				{Sub: "admin", Dom: "tenant-acme", Obj: "ledger", Act: "write", Effect: "allow"},
				{Sub: "viewer", Dom: "tenant-acme", Obj: "ledger", Act: "read", Effect: "allow"},
			},
			expectedGrouping: []rbac.GroupingRule{
				{User: "alice", Role: "admin", Dom: "tenant-acme"},
				{User: "bob", Role: "viewer", Dom: "tenant-acme"},
			},
			expectedError: nil,
		},
		{
			name:             "empty table loads empty sets",
			seed:             false,
			expectedPolicies: nil,
			expectedGrouping: nil,
			expectedError:    nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := openPolicyDB(t)

			if tc.seed {
				seedCasbinRows(t, db)
			}

			loader, err := rbac.NewDBPolicyLoader(db)
			require.NoError(t, err)

			policies, grouping, err := loader.Load()
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.expectedPolicies, policies)
			assert.ElementsMatch(t, tc.expectedGrouping, grouping)
		})
	}
}

func TestDBPolicyLoaderHotReload(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	db := openPolicyDB(t)
	seedCasbinRows(t, db)

	loader, err := rbac.NewDBPolicyLoader(db)
	require.NoError(t, err)

	enforcer, err := rbac.NewEnforcer(rbac.RBACParams{
		Loader:               loader,
		ThresholdAmountMinor: 100000,
	})
	require.NoError(t, err)

	alice := appport.Subject{ID: "alice", TenantID: valueobject.TenantID("tenant-acme")}
	assert.NoError(t, enforcer.Authorize(context.Background(), alice, "write", "ledger"))

	bob := appport.Subject{ID: "bob", TenantID: valueobject.TenantID("tenant-acme")}
	assert.ErrorIs(t, enforcer.Authorize(context.Background(), bob, "write", "ledger"), rbac.ErrForbidden)

	require.NoError(t, db.WithContext(context.Background()).Exec(
		"INSERT INTO casbin_rules (ptype, v0, v1, v2, v3, v4) VALUES ('p', 'viewer', 'tenant-acme', 'ledger', 'write', 'allow')",
	).Error)

	require.NoError(t, enforcer.ReloadFrom(loader))
	assert.NoError(t, enforcer.Authorize(context.Background(), bob, "write", "ledger"))
}
