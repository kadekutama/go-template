package migration_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kadekutama/go-template/internal/infrastructure/database/migration"
)

func TestMigrationListNumbering(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name           string
		expectedCount  int
		expectedFirst  int
		expectedSecond int
		expectedThird  int
		expectedFourth int
	}

	testCases := []testCase{
		{
			name:           "core versions numbered with zero gaps",
			expectedCount:  4,
			expectedFirst:  1,
			expectedSecond: 2,
			expectedThird:  3,
			expectedFourth: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := migration.NewRunner(migration.RunnerParams{DB: nil})
			require.Error(t, err, "nil DB must fail")
			assert.Nil(t, runner)

			migrations, err := migration.ListPairs(migration.Versions, "versions")
			require.NoError(t, err)
			require.Len(t, migrations, tc.expectedCount)
			assert.Equal(t, tc.expectedFirst, migrations[0].Version)
			assert.Equal(t, tc.expectedSecond, migrations[1].Version)
			assert.Equal(t, tc.expectedThird, migrations[2].Version)
			assert.Equal(t, tc.expectedFourth, migrations[3].Version)
			assert.NotEmpty(t, migrations[0].UpSQL)
			assert.NotEmpty(t, migrations[0].DownSQL)
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
			file: "versions/000001_ledger_core.up.sql",
			mustContain: []string{
				"BIGINT",
				"NUMERIC(38,0)",
				"prevent_posted_mutation",
				"check_posting_balanced",
				"tenant_id",
				"ledger_seq",
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
			file: "versions/000002_idempotency_outbox.up.sql",
			mustContain: []string{
				"idempotency_records",
				"outbox_events",
				"PRIMARY KEY (tenant_id, key)",
				"UNIQUE (aggregate_id, aggregate_version)",
			},
			mustNotMatch: []string{
				"FLOAT",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := migration.Versions.ReadFile(tc.file)
			require.NoError(t, err)

			upper := strings.ToUpper(string(raw))

			for _, want := range tc.mustContain {
				assert.Contains(t, upper, strings.ToUpper(want))
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
		name string
		file string
		want []string
	}

	testCases := []testCase{
		{
			name: "core down drops every created table",
			file: "versions/000001_ledger_core.down.sql",
			want: []string{"checkpoints", "holds", "entries", "postings", "accounts", "assets", "ledgers"},
		},
		{
			name: "idempotency down drops both tables",
			file: "versions/000002_idempotency_outbox.down.sql",
			want: []string{"outbox_events", "idempotency_records"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := migration.Versions.ReadFile(tc.file)
			require.NoError(t, err)

			for _, table := range tc.want {
				assert.Contains(t, string(raw), table)
			}
		})
	}
}
