package migration_test

import (
	"context"
	"database/sql"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
	"github.com/kadekutama/go-template/internal/shared/kernel/log"
	"github.com/kadekutama/go-template/test/testcontainers"
)

// noFatalLogger proves the goose bridge never terminates the process: any
// Fatalf mapping would panic the test instead of exiting.
type noFatalLogger struct{}

func (noFatalLogger) Trace(context.Context, string, ...any) {}
func (noFatalLogger) Debug(context.Context, string, ...any) {}
func (noFatalLogger) Info(context.Context, string, ...any)  {}
func (noFatalLogger) Warn(context.Context, string, ...any)  {}
func (noFatalLogger) Error(context.Context, string, ...any) {}
func (noFatalLogger) Panic(_ context.Context, msg string, _ ...any) {
	panic(msg)
}
func (noFatalLogger) Fatal(_ context.Context, msg string, _ ...any) {
	panic("goose bridge must never exit: " + msg)
}
func (noFatalLogger) With(...any) log.Logger { return noFatalLogger{} }

func TestNewRunner(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		params        migration.RunnerParams
		expectedError string
	}

	testCases := []testCase{
		{
			name: "nil database handle rejected",
			params: migration.RunnerParams{
				DB: nil,
			},
			expectedError: "migration: DB is required",
		},
		{
			name: "invalid subdirectory path rejected",
			params: func() migration.RunnerParams {
				db, err := sql.Open("pgx", "postgres://localhost:5432/test?sslmode=disable")
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				return migration.RunnerParams{
					DB:  db,
					FS:  fstest.MapFS{},
					Dir: "/invalid_leading_slash",
				}
			}(),
			expectedError: "migration: sub fs /invalid_leading_slash",
		},
		{
			name: "unreachable database fails inside bounded bootstrap",
			params: func() migration.RunnerParams {
				db, err := sql.Open("pgx", "postgres://postgres:pw@192.0.2.1:5432/test?sslmode=disable")
				require.NoError(t, err)
				t.Cleanup(func() { _ = db.Close() })
				return migration.RunnerParams{
					DB:      db,
					Timeout: 3 * time.Second,
				}
			}(),
			expectedError: "migration: bootstrap legacy ledger",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := migration.NewRunner(tc.params)
			require.Error(t, err)
			assert.Nil(t, runner)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestMigrationListVersions(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		expectedCount    int
		expectedVersions []int64
	}

	testCases := []testCase{
		{
			name:             "timestamped baseline plus citus distribution in order",
			expectedCount:    5,
			expectedVersions: []int64{20260901000001, 20260901000002, 20260901000003, 20260901000004, 20260901000005},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			migrations, err := migration.List(migration.Versions, "versions")
			require.NoError(t, err)
			require.Len(t, migrations, tc.expectedCount)

			actual := make([]int64, 0, len(migrations))
			for _, m := range migrations {
				actual = append(actual, m.Version)
				assert.NotEmpty(t, m.Name)
				assert.NotEmpty(t, m.Source)
			}

			assert.Equal(t, tc.expectedVersions, actual)

			aliased, err := migration.ListPairs(migration.Versions, "versions")
			require.NoError(t, err)
			assert.Equal(t, migrations, aliased)
		})
	}
}

func TestMigrationListInvalid(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		files         map[string]string
		expectedError string
	}

	testCases := []testCase{
		{
			name: "bad file name rejected",
			files: map[string]string{
				"versions/not_a_migration.sql": "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
			},
			expectedError: "migration: bad file name",
		},
		{
			name: "missing down block rejected",
			files: map[string]string{
				"versions/20260901000001_only_up.sql": "-- +goose Up\nSELECT 1;\n",
			},
			expectedError: "needs non-empty Up and Down blocks",
		},
		{
			name: "legacy numbering gap rejected",
			files: map[string]string{
				"versions/000001_first.sql":  "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
				"versions/000002_second.sql": "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
				"versions/000004_fourth.sql": "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
			},
			expectedError: "legacy numbering gaps",
		},
		{
			name: "duplicate versions rejected",
			files: map[string]string{
				"versions/20260901000001_first.sql":  "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
				"versions/20260901000001_second.sql": "-- +goose Up\nSELECT 1;\n-- +goose Down\nSELECT 1;\n",
			},
			expectedError: "migration: duplicate version",
		},
		{
			name:          "empty directory rejected",
			files:         map[string]string{},
			expectedError: "no .sql migration files found",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mapFS := fstest.MapFS{}
			for name, content := range tc.files {
				mapFS[name] = &fstest.MapFile{Data: []byte(content)}
			}
			if len(tc.files) == 0 {
				mapFS["versions"] = &fstest.MapFile{Mode: fs.ModeDir}
			}

			migrations, err := migration.List(mapFS, "versions")
			require.Error(t, err)
			assert.Nil(t, migrations)
			assert.Contains(t, err.Error(), tc.expectedError)
		})
	}
}

func TestMigrationSQLContent(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		file         string
		mustContain  []string
		mustNotMatch []string
	}

	testCases := []testCase{
		{
			name: "ledger core uses integer minor units and numeric aggregates",
			file: "versions/20260901000001_ledger_core.sql",
			mustContain: []string{
				"BIGINT",
				"NUMERIC(38,0)",
				"prevent_posted_mutation",
				"check_posting_balanced",
				"tenant_id",
				"ledger_seq",
				"-- +goose Up",
				"-- +goose Down",
				"-- +goose StatementBegin",
				"-- +goose StatementEnd",
			},
			mustNotMatch: []string{
				"FLOAT",
				"DOUBLE",
				"REAL ",
				"balance_minor BIGINT",
			},
		},
		{
			name: "idempotency and outbox tables keyed correctly",
			file: "versions/20260901000002_idempotency_outbox.sql",
			mustContain: []string{
				"idempotency_records",
				"outbox_events",
				"PRIMARY KEY (tenant_id, key)",
				"PRIMARY KEY (tenant_id, id)",
				"UNIQUE (tenant_id, aggregate_id, aggregate_version)",
				"-- +goose Up",
				"-- +goose Down",
			},
			mustNotMatch: []string{
				"FLOAT",
			},
		},
		{
			name: "citus distribution runs outside transactions",
			file: "versions/20260901000005_citus_distribution.sql",
			mustContain: []string{
				"-- +goose NO TRANSACTION",
				"-- +goose Up",
				"-- +goose Down",
				"create_distributed_table",
				"create_reference_table",
			},
			mustNotMatch: []string{},
		},
		{
			name: "rls policies wrap dynamic blocks in statement delimiters",
			file: "versions/20260901000004_rls_policies.sql",
			mustContain: []string{
				"-- +goose Up",
				"-- +goose Down",
				"-- +goose StatementBegin",
				"-- +goose StatementEnd",
				"tenant_isolation",
			},
			mustNotMatch: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := migration.Versions.ReadFile(tc.file)
			require.NoError(t, err)

			upper := strings.ToUpper(string(raw))

			for _, requiredToken := range tc.mustContain {
				assert.Contains(t, upper, strings.ToUpper(requiredToken))
			}

			code := stripSQLComments(string(raw))
			upperCode := strings.ToUpper(code)

			for _, banned := range tc.mustNotMatch {
				assert.NotContains(t, upperCode, strings.ToUpper(banned), "banned token %s in %s", banned, tc.file)
			}
		})
	}
}

// stripSQLComments removes -- line comments so content scans inspect code.
func stripSQLComments(sql string) string {
	lines := strings.Split(sql, "\n")
	kept := make([]string, 0, len(lines))

	for _, line := range lines {
		if index := strings.Index(line, "--"); index >= 0 {
			line = line[:index]
		}

		kept = append(kept, line)
	}

	return strings.Join(kept, "\n")
}

func TestMigrationDownReversible(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		file           string
		expectedTables []string
	}

	testCases := []testCase{
		{
			name:           "core down drops every created table",
			file:           "versions/20260901000001_ledger_core.sql",
			expectedTables: []string{"checkpoints", "holds", "entries", "postings", "accounts", "assets", "ledgers"},
		},
		{
			name:           "idempotency down drops both tables",
			file:           "versions/20260901000002_idempotency_outbox.sql",
			expectedTables: []string{"outbox_events", "idempotency_records"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := migration.Versions.ReadFile(tc.file)
			require.NoError(t, err)

			downIndex := strings.Index(string(raw), "-- +goose Down")
			require.GreaterOrEqual(t, downIndex, 0, "unified file must carry a Down block")

			down := string(raw)[downIndex:]
			for _, table := range tc.expectedTables {
				assert.Contains(t, down, table)
			}
		})
	}
}

func TestLegacyBootstrap(t *testing.T) {
	testcontainers.SkipIfNoDocker(t)

	ctx := context.Background()

	handle, err := testcontainers.StartPostgres(t, "ledger_bootstrap")
	require.NoError(t, err)

	sqlDB, err := sql.Open("pgx", handle.ConnectionString())
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	_, err = sqlDB.ExecContext(ctx, `CREATE TABLE schema_migrations (
    version    INTEGER NOT NULL PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
)`)
	require.NoError(t, err)

	_, err = sqlDB.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES (1), (2), (3), (4)`)
	require.NoError(t, err)

	first, err := migration.NewRunner(migration.RunnerParams{DB: sqlDB, Logger: noFatalLogger{}})
	require.NoError(t, err)

	var copied []int64

	rows, err := sqlDB.QueryContext(ctx, `SELECT version_id FROM goose_db_version ORDER BY version_id`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var version int64
		require.NoError(t, rows.Scan(&version))
		copied = append(copied, version)
	}

	require.NoError(t, rows.Err())
	assert.Equal(t, []int64{1, 2, 3, 4}, copied, "bootstrap must copy legacy versions 1..4")

	second, err := migration.NewRunner(migration.RunnerParams{DB: sqlDB})
	require.NoError(t, err)

	var count int
	require.NoError(t, sqlDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM goose_db_version`).Scan(&count))
	assert.Equal(t, 4, count, "repeated pod boots must not duplicate ledger rows")

	require.NoError(t, second.Up(ctx))

	version, err := second.Version(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(20260901000005), version)

	require.NoError(t, first.Up(ctx), "re-running Up must stay idempotent")
}
