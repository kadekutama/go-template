// Package migration applies embedded Goose v3 SQL migrations with an
// automatic idempotent bootstrap from the legacy schema_migrations ledger
// (E07.1-T01, ADR-018). Baseline files 000001–000004 keep their numeric
// prefixes; newer files use 14-digit UTC timestamps. Up applies pending
// versions in order (out-of-order allowed); Down rolls back the latest one.
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pressly/goose/v3"

	"github.com/kadekutama/go-template/internal/shared/kernel/log"
)

var gooseFile = regexp.MustCompile(`^(\d{6}|\d{14})_[a-z0-9_]+\.sql$`)

// BootstrapDefaultTimeout bounds the legacy-ledger bootstrap when
// RunnerParams carries no explicit Timeout.
const BootstrapDefaultTimeout = 30 * time.Second

// Migration is one versioned Goose SQL file.
type Migration struct {
	Version int64
	Name    string
	Source  string
}

// RunnerParams carries Runner dependencies (Parameter Object pattern).
// Logger receives goose operational output and stays nil-tolerant: a nil
// logger keeps goose silent. Timeout bounds the legacy bootstrap only;
// non-positive selects BootstrapDefaultTimeout. Up/Down honor the caller's
// context instead.
type RunnerParams struct {
	DB      *sql.DB
	FS      fs.FS
	Dir     string
	Logger  log.Logger
	Timeout time.Duration
}

// Runner applies Goose migrations from an embedded FS directory.
type Runner struct {
	fsys     fs.FS
	dir      string
	provider *goose.Provider
}

// NewRunner builds a Runner; DB must be non-nil. It bootstraps the legacy
// schema_migrations ledger into goose_db_version once, idempotently, so
// existing databases never replay migrations 1..4 on upgrade.
func NewRunner(params RunnerParams) (*Runner, error) {
	if params.DB == nil {
		return nil, fmt.Errorf("migration: DB is required")
	}

	fsys := params.FS
	if fsys == nil {
		fsys = Versions
	}

	dir := params.Dir
	if dir == "" {
		dir = "versions"
	}

	sub, err := subFS(fsys, dir)
	if err != nil {
		return nil, err
	}

	timeout := resolveTimeout(params.Timeout)
	bootCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := bootstrapLegacyLedger(bootCtx, params.DB); err != nil {
		return nil, fmt.Errorf("migration: bootstrap legacy ledger: %w", err)
	}

	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		params.DB,
		sub,
		goose.WithAllowOutofOrder(true),
		goose.WithLogger(gooseLog(params.Logger)),
	)
	if err != nil {
		return nil, fmt.Errorf("migration: init goose provider: %w", err)
	}

	return &Runner{fsys: fsys, dir: dir, provider: provider}, nil
}

// gooseLogger forwards goose operational output to the kernel log port.
// Fatalf maps to an error line: library code must never terminate the
// process, so a goose fatal is reported, never acted on.
type gooseLogger struct {
	logger log.Logger
}

func (l gooseLogger) Printf(format string, values ...any) {
	l.logger.Info(context.Background(), fmt.Sprintf(format, values...))
}

func (l gooseLogger) Fatalf(format string, values ...any) {
	l.logger.Error(context.Background(), fmt.Sprintf(format, values...))
}

// gooseLog resolves the goose logger: the kernel-backed bridge when a logger
// is configured, silence otherwise.
func gooseLog(logger log.Logger) goose.Logger {
	if logger == nil {
		return goose.NopLogger()
	}

	return gooseLogger{logger: logger}
}

// resolveTimeout normalizes the bootstrap timeout: non-positive selects
// BootstrapDefaultTimeout so callers get a bounded bootstrap by default.
func resolveTimeout(timeout time.Duration) time.Duration {
	if timeout <= 0 {
		return BootstrapDefaultTimeout
	}

	return timeout
}

// subFS resolves dir inside fsys ("." means the FS root itself).
func subFS(fsys fs.FS, dir string) (fs.FS, error) {
	if dir == "." || dir == "" {
		return fsys, nil
	}

	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migration: sub fs %s: %w", dir, err)
	}

	return sub, nil
}

// bootstrapLegacyLedger copies applied versions from the bespoke
// schema_migrations table into goose_db_version exactly once. The
// WHERE NOT EXISTS guard keeps repeated multi-pod boots duplicate-free.
func bootstrapLegacyLedger(ctx context.Context, db *sql.DB) error {
	const query = `
DO $goose_bootstrap$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'schema_migrations') THEN
        CREATE TABLE IF NOT EXISTS goose_db_version (
            id serial PRIMARY KEY,
            version_id bigint NOT NULL,
            is_applied boolean NOT NULL,
            tstamp timestamp DEFAULT now()
        );
        CREATE UNIQUE INDEX IF NOT EXISTS goose_db_version_version_id_idx ON goose_db_version (version_id);
        INSERT INTO goose_db_version (version_id, is_applied)
        SELECT sm.version, true
        FROM schema_migrations sm
        WHERE NOT EXISTS (
            SELECT 1 FROM goose_db_version gv WHERE gv.version_id = sm.version
        );
    END IF;
END
$goose_bootstrap$;`

	if _, err := db.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("migration: copy legacy ledger: %w", err)
	}

	return nil
}

// List returns the ordered migrations in the runner's directory.
func (r *Runner) List() ([]Migration, error) {
	return List(r.fsys, r.dir)
}

// List parses unified Goose files from any FS without a database
// connection: legacy 000001–000004 stay gapless, newer files use 14-digit
// UTC timestamps in ascending order, and every file carries Up+Down blocks.
func List(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("migration: read dir: %w", err)
	}

	parsed := make([]Migration, 0, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		m, err := parseGooseFile(fsys, dir, entry.Name())
		if err != nil {
			return nil, err
		}

		parsed = append(parsed, m)
	}

	return orderGoose(parsed)
}

// ListPairs is the backward-compatible alias for List.
func ListPairs(fsys fs.FS, dir string) ([]Migration, error) {
	return List(fsys, dir)
}

// parseGooseFile validates one file name and its Up/Down annotations.
func parseGooseFile(fsys fs.FS, dir, filename string) (Migration, error) {
	match := gooseFile.FindStringSubmatch(filename)
	if match == nil {
		return Migration{}, fmt.Errorf("migration: bad file name %s", filename)
	}

	version, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil {
		return Migration{}, fmt.Errorf("migration: bad version %s: %w", filename, err)
	}

	raw, err := fs.ReadFile(fsys, dir+"/"+filename)
	if err != nil {
		return Migration{}, fmt.Errorf("migration: read %s: %w", filename, err)
	}

	up, down := splitGooseBlocks(string(raw))
	if strings.TrimSpace(up) == "" || strings.TrimSpace(down) == "" {
		return Migration{}, fmt.Errorf("migration: %s needs non-empty Up and Down blocks", filename)
	}

	name := strings.TrimSuffix(filename, ".sql")

	return Migration{Version: version, Name: name, Source: filename}, nil
}

// splitGooseBlocks separates the Up and Down sections at their annotations.
func splitGooseBlocks(raw string) (string, string) {
	upIndex := strings.Index(raw, "-- +goose Up")
	downIndex := strings.Index(raw, "-- +goose Down")

	if upIndex < 0 || downIndex < 0 || downIndex < upIndex {
		return "", ""
	}

	return raw[upIndex+len("-- +goose Up") : downIndex], raw[downIndex+len("-- +goose Down"):]
}

// orderGoose sorts by version and enforces gapless legacy numbering for any
// 6-digit files still present (pre-release baselines keep theirs; all new
// files use 14-digit UTC timestamps in ascending order).
func orderGoose(migrations []Migration) ([]Migration, error) {
	if len(migrations) == 0 {
		return nil, fmt.Errorf("migration: no .sql migration files found")
	}

	sort.Slice(migrations, func(i, j int) bool { return migrations[i].Version < migrations[j].Version })

	for index := 1; index < len(migrations); index++ {
		if migrations[index].Version == migrations[index-1].Version {
			return nil, fmt.Errorf("migration: duplicate version %d (%s and %s)",
				migrations[index].Version, migrations[index-1].Source, migrations[index].Source)
		}
	}

	var legacy []int64

	for _, m := range migrations {
		if m.Version >= 1 && m.Version <= 999999 {
			legacy = append(legacy, m.Version)
		}
	}

	for index, version := range legacy {
		if version != int64(index+1) {
			return nil, fmt.Errorf("migration: legacy numbering gaps: %v", legacy)
		}
	}

	return migrations, nil
}

// Up applies every pending version in order.
func (r *Runner) Up(ctx context.Context) error {
	if _, err := r.provider.Up(ctx); err != nil {
		return fmt.Errorf("migration: apply up: %w", err)
	}

	return nil
}

// Down rolls back the latest applied version. On an empty database the
// provider reports ErrNoNextVersion, preserving the legacy "nothing applied"
// failure instead of a silent success.
func (r *Runner) Down(ctx context.Context) error {
	if _, err := r.provider.Down(ctx); err != nil {
		return fmt.Errorf("migration: rollback: %w", err)
	}

	return nil
}

// Version returns the highest applied version (0 when none).
func (r *Runner) Version(ctx context.Context) (int64, error) {
	version, err := r.provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("migration: current version: %w", err)
	}

	return version, nil
}
